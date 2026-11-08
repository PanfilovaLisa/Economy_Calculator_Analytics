# Economy_Calculator_Analytics

```bash
cd Economy_Calculator_Analytics
cd backend
go build ./...
go run cmd/main.go

#в другом терминале
cd Economy_Calculator_Analytics
curl -X POST http://localhost:8082/calculate -F "file=@testfile/1deal.csv"
```

# Cashflow MVP

## Общая идея

Генератор строит **три независимых расписания дат**:

1. **Тело (principal)** — когда уменьшается остаток долга.
2. **Проценты (interest)** — когда списываются начисленные проценты.
3. **Переоценка (repricing)** — когда обновляется ставка (только для FLOAT и sub-ставок).

На пересечении дат эмитятся строки в `CashFlow`. Проценты начисляются **непрерывно** между своими датами, но **эмитятся** только в даты процентов. Тело уменьшает остаток **только** в свои даты. Переоценка **не эмитит** строк — она **меняет** ставку для периодов **после** своей даты.

Остаток `remaining` **общий**: он уменьшается на датах тела. Проценты на своей дате считаются от `remaining` **на начало периода** (после всех тел, которые были **до** этой даты). Тело и проценты в один день: **сначала проценты от остатка до тела**, потом тело — как в текущей реализации.

Для MVP — упрощение: проценты за период считаются от остатка на начало периода (без разбивки по датам тела и переоценки). Это даёт приблизительный результат для сделок, где тело и переоценка не попадают внутрь периода процентов. Для точного расчёта — разбивать период по датам тела и переоценки, считать каждую часть отдельно.

---

## Часть 1. Тело (principal)

### Приоритет источников

1. **`principal_payment`** — если заполнен, это явный список дат и сумм. Приоритетнее правила.
2. **`regular_payment_rule_period_cd`** — если `principal_payment` пуст:
   - **`ENDDATE`** → одна дата `maturity_dttm`, вся сумма остатка.
   - **любое другое** (`1M`, `2M`, `3M`, `45D`, `1Y`) → правило от `regular_payment_rule_start_dttm` с шагом `period_cd`.
3. **Fallback** — если всё пусто → график процентов (тело гасится в даты процентов).

### Как строить правило

- **`period_cd`**: любое значение с суффиксом `M` / `D` / `Y` — валидно. `ENDDATE` — особый случай.
- **Шаг**: `M` → +N месяцев, `D` → +N дней, `Y` → +N лет. Месяцы и годы — с клампингом (31-е в феврале → 28/29).
- **`regular_payment_rule_day_num`**:
  - **задан** → первый платёж ровно `start_dttm`, все последующие — в этот день месяца. `-1` = последний день месяца.
  - **не задан** → первый `start_dttm`, все последующие — `start_dttm + N×period`.
- **Граница**: даты до `maturity_dttm` включительно. Если последняя дата раньше `maturity` — добавляем `maturity` отдельной датой (`close_payment`).

### Суммы тела

- **ANN** — фиксированный платёж `PMT` = `P × r / (1 − (1+r)^(−n))`, где `P` — остаток, `r` — ставка за период (годовая / число периодов в году), `n` — число оставшихся платежей. Тело на каждой дате = `PMT − interest`, `interest` считается на эту же дату.  
- **RGM** — `P / n`, последняя дата добивает остаток.
- **SIMPLE** — если `principal_payment` явный → суммы из списка; если правило → тело в конце (как `ENDDATE`).

### Особые случаи

- Даты в прошлом (до `report_dttm`) — пропускаем.
- Суммы `principal_payment` не сходятся с `current_principal_amt` — последняя дата добивает остаток, предупреждение в лог.
- `maturity` не попала в график — добавляем `close_payment` с остатком.

---

## Часть 2. Проценты (interest)

### Приоритет источников дат

1. **`interest_payment`** — если заполнен, явный список дат (без сумм). Приоритетнее правила.
2. **`interest_payment_rule_period_cd`** — если `interest_payment` пуст:
   - **`ENDDATE`** → одна дата `maturity_dttm`, все проценты за срок.
   - **любое другое** → правило от `interest_payment_rule_start_dttm` с шагом `period_cd`.
3. **Fallback** — одна дата `maturity` (все проценты в конце).

### Формула процентов на дату
```
interest = remaining × rate × YearFraction(prev_date, date, day_convention) 
```

- `remaining` — остаток на начало периода (после всех тел **до** `prev_date`).
- `rate` — эффективная ставка на этот период (с учётом переоценки).
- `YearFraction`:
  - **`ACT/365`** → `days / 365`.
  - **`ACT/ACT ISDA`** → по календарным годам: дни в первом году / дней в первом году + дни во втором / дней во втором. Исправить асимметрию `+1`.
  - **`ACT/ACT`** (без ISDA) → уточнить; для MVP — как ACT/365.
  - **Другое** → ACT/365 (fallback).
- Первый период — от `max(report_dttm, deal_dttm)` до первой даты процентов. Даты в прошлом пропускаем.

### Ставка — FIXED

- `rate = interest_rate`, затем зажимаем в `[floor, cap]` (если заданы).
- **`repricing_dttm_rate_schedule_fixed`** — явное расписание ступеней (пары «дата, ставка»). Приоритетнее `interest_rate`.
  - `repricing_dttm_schedule` — даты переоценки; должны совпадать с датами в `_fixed`. Если не совпадают — используем `_fixed`, предупреждение в лог.
  - `repricing_dttm_rate_schedule` (без `_fixed`) для FIXED не используется — это для FLOAT.
- Sub-ставка — см. Часть 4.

### Ставка — FLOAT

- `rate = interest_rate + base_indicator_rate` (маржа + значение индекса на дату отчёта).
- `base_indicator` — имя индекса (`CBKEY1M`), значение из заглушки.
- `cap_limit` / `floor_limit` — зажимаем после сложения.

- Если `base_indicator_rate` пуст — `rate = interest_rate`
- **Если расписание переоценки отсутствует** (нет ни `repricing_dttm_schedule`, ни `repricing_rule_*`, ни `repricing_dttm_rate_schedule`) → ставка **фиксирована** на уровне `interest_rate + base_indicator_rate` (если `base_indicator_rate` задан) или `interest_rate` (если пуст). Это не «FIXED» — тип остаётся FLOAT, но без обновления индекса.

> Точное поведение при отсутствии переоценки — уточнить у бизнеса. Для MVP — как описано выше.

### Расписание переоценки (FLOAT)

Приоритет источников:

1. **`repricing_rule_start_dttm` + `repricing_rule_period_cd` + `repricing_rule_day_num`** — правило (как для тела). Если `period_cd` пуст, а `start` заполнен — одна переоценка в `start`.
2. **`repricing_dttm_schedule`** — расписание дат. Используется вместе с `repricing_dttm_rate_schedule`, если оба заполнены (даты — из schedule, ставки — из rate_schedule). Если только `_schedule` — используем его как источник дат. Новые ставки берем из заглушки

На каждой дате переоценки:

- Если есть `repricing_dttm_rate_schedule` — берём ставку оттуда (плюс маржа, если она не включена в значение).
- Иначе — берём новое значение `CBKEY1M` из заглушки на эту дату + `interest_rate`.
- Затем зажимаем в `[floor, cap]`.

### Заглушка `CBKEY1M` для MVP

- Файл `cbkey.csv` с колонками «дата, значение». Читается при старте. Пример:
```
date;rate
01.01.2022;0.085
15.09.2022;0.075
15.12.2022;0.070
```
- Функция «получить `CBKEY1M` на дату» → последнее значение до или в эту дату. Если дата раньше первой — первое значение, предупреждение.
- Имитирует API ЦБ. В будущем — заменить на реальный API.

---

## Часть 3. Переоценка (repricing)

**Переоценка не эмитит строк в `CashFlow`.** Это вспомогательное расписание, которое влияет на расчёт процентов. Её задача — определить, какая ставка действует в каждый момент времени.

### Алгоритм

1. Построить расписание переоценки — массив дат (только для FLOAT и sub-ставок).
2. Для каждой даты процентов — найти последнюю переоценку до или в эту дату. Использовать её ставку.
3. Если переоценок нет — использовать базовую ставку:
   - для FIXED — `interest_rate` (или ступень из `_fixed`);
   - для FLOAT — `interest_rate + base_indicator_rate`.

### Порядок обработки в один день

Если тело и проценты приходятся на одну дату:

- Сначала считаем проценты **от остатка до тела** (`balanceBefore`), эмитим строку `interest_payment` с `RemainingBalance = balanceBefore`.
- Затем применяем тело, эмитим строку `principal_payment` с `RemainingBalance = balanceAfter`.

---

## Часть 4. Sub-ставка (подчинённая)

### Что это

Вторая ставка поверх основной. Применяется, если `sub_rate_type_cd` заполнен.

- **`FIXED`** → `sub_rate = sub_interest_rate`.
- **`FLOAT`** → `sub_rate = sub_interest_rate + sub_float_indicator` (значение из заглушки, если `sub_float_indicator = CBKEY1M`).
- `sub_cap_limit` / `sub_floor_limit` — зажимаем `sub_rate` в `[sub_floor, sub_cap]`.
- `sub_coeff_index` — **множитель** для sub-ставки: `sub_rate_итог = sub_rate × sub_coeff_index`. Например, `sub_interest_rate = 0.10`, `sub_coeff_index = 0.9` → `sub_rate_итог = 0.09` (90% от sub-ставки). Если `sub_coeff_index` пуст — коэффициент = `1.0` (без масштабирования). Применяется **после** `sub_cap` / `sub_floor`, **до** добавления к основной ставке.
- `sub_repricing_*` — правило переоценки sub-ставки (как основное).

### Применение

**Для MVP: `sub_rate` добавляется к основной ставке.**

Итоговая ставка = `interest_rate + sub_rate_итог`, где `sub_rate_итог` — после применения `sub_cap` / `sub_floor` и `sub_coeff_index`. Если `sub_rate_type_cd` пуст — `sub_rate = 0`, ставка = `interest_rate`.

Точная логика применения (заменять / добавлять / к части остатка) — уточнить у бизнеса. Переключение на другой вариант — одна строка в коде.

### Приоритет

- `sub_repricing_rule_*` — переоценка sub-ставки.
- `sub_payment_rule_*` — не отдельный график платежей в MVP; для упрощения sub-ставка переоценивается в те же даты, что основная, либо по своему правилу.

---

## Часть 5. Общие правила

### Сводка приоритетов

| Что | Приоритет 1 | Приоритет 2 | Fallback |
|---|---|---|---|
| Тело | `principal_payment` | `regular_payment_rule_*` | график процентов |
| Проценты | `interest_payment` | `interest_payment_rule_*` | `maturity` |
| Переоценка FLOAT | `repricing_dttm_schedule` (+ `rate_schedule`) | `repricing_rule_*` | нет переоценки |
| Ставка FIXED | `repricing_dttm_rate_schedule_fixed` | `interest_rate` | — |
| Sub-переоценка | `sub_repricing_rule_*` | — | как основная |

### Даты в прошлом

Пропускаем (до `report_dttm`). Первый будущий период — от `report_dttm`.

### `maturity` не попала в график

Добавляем `maturity` отдельной датой:

- тело — `close_payment` с остатком (если остаток не 0);
- проценты — `interest_payment` за период от последней даты до `maturity`.

### Депозит (`current_principal_amt < 0`)

Все суммы умножаются на `-1`. Проценты и тело считаются по модулю остатка.

### `remaining` не закрылся

Добиваем на последней дате (`maturity` или последняя дата графика).

### Неизвестные значения

- `rate_type_cd` не FIXED и не FLOAT → fallback FIXED, предупреждение.
- `day_convention_cd` не ACT/365, и не ACT/ACT, и не ACT/ACT ISDA → fallback ACT/365, предупреждение.

---

## Часть 6. Что не делаем в MVP

1. `behavior_model_cd` — модель досрочного погашения, отдельный слой.
2. `repricing_dttm_rate_schedule` (без `_fixed`) для FIXED — только `_fixed`.
3. `sub_payment_rule_*` — не отдельный график sub-платежей.
4. `indicator_lag_cd` / `sub_indicator_lag_cd` — лаг индекса.
5. `accrued_interest` — не в генерации (справочно).
6. `start_principal_amt` — только `current_principal_amt`.
7. `product`, `deal_report_dttm` — метаданные.

---

## Часть 7. Порядок реализации

### Этап 1 — каркас

1. `flowContext` — добавить `Now`, `DayConvention`, `Rate`, `SubRate`, `IsDeposit`, метод `isDeposit()`.
2. `prepareContext` — валидация, `StartDate = max(report, deal)`, `Sign`, базовая `Rate`.
3. `buildSchedule` — общая функция для любого графика (тело, проценты, переоценка): `start`, `period`, `day_num`, `maturity`, возвращает `[]time.Time`.
4. `parseManualSchedule` — для `principal_payment` (даты + суммы) и `interest_payment` (только даты).

### Этап 2 — три расписания

1. `buildPrincipalDates` — приоритет: `principal_payment` → `regular_*` → fallback.
2. `buildInterestDates` — приоритет: `interest_payment` → `interest_*` → `maturity`.
3. `buildRepricingDates` — только для FLOAT: `repricing_dttm_schedule` → `repricing_rule_*`.
4. `GenerateCashFlows` — диспетчер: собирает три расписания, объединяет даты, эмитит строки.

### Этап 3 — ставки

1. `effectiveRate` — FIXED (`interest_rate` + cap/floor), FLOAT (`interest_rate + base_indicator_rate` + cap/floor).
2. `rateAt(date)` — ставка на дату с учётом переоценки (FLOAT) или ступеней (FIXED `_fixed`).
3. Заглушка `CBKEY1M` — чтение файла, `getCBKEY(date)`.

### Этап 4 — тело по типам

1. `generateAnnuityFlows` — PMT, `principal = PMT − interest`.
2. `generateDifferentiatedFlows` — `principal = P / n`.
3. `generateSimpleFlows` — `principal_payment` явный → суммы из списка; иначе — в конце.

### Этап 5 — sub-ставка

1. `subRateAt(date)` — по `sub_*`.
2. Применение — добавлять к основной (уточнить у бизнеса).

### Этап 6 — тесты и краевые случаи

1. Тест на каждую сделку из `тестовые сделки МАИ.csv` — проверить инварианты:
   - Σ principal = `current_principal_amt` (с учётом знака);
   - последний `remaining = 0`;
   - первый interest = `remaining × rate × days/365`.
2. Краевые случаи: `principal_payment` не сходится, `maturity` не в графике, `rate_type` неизвестен, `day_convention` неизвестен.

---
## Сделки, которые правильно работают
1, 2, 5, 7, 13, 15

## Замечания
* округление - нужно продумать

* дата отчета не значит что в ней обязательно должен происходить платеж! (пример 1 сделка)
для решения добавляю поле AccrualStart  time.Time в flowContext  
? что переделать для simple?

* ошибка: если генерация идет вроде remaining > 0, нету close payment
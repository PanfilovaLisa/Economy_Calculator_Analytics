package cashflow

import "time"

// Типы строк в выходном CashFlow
const (
	TypeInterestPayment  = "interest_payment"  // платёж процентов
	TypePrincipalPayment = "principal_payment" // платёж тела долга (регулярный)
	TypePrepayment       = "prepayment"        // досрочное погашение тела (ручной график)
	TypeClosePayment     = "close_payment"     // техническая строка закрытия сделки
	TypeRepricing        = "repricing"         // переоценка ставки (зарезервировано)
	TypeTimeBucketEnd    = "time_bucket_end"   // конец временного бакета (зарезервировано)
)

// Типы сделок
const (
	DiscANN    = "ANN"    // аннуитет: равный суммарный платёж
	DiscRGM    = "RGM"    // дифференцированный: равное тело, проценты на остаток
	DiscSIMPLE = "SIMPLE" // простой: тело в конце или по ручному графику
)

type Deal struct {
	ReportDate    time.Time `json:"report_dttm"`   // дата снимка портфеля
	Discriminator string    `json:"discriminator"` // тип сделки: ANN / RGM / SIMPLE
	DealID        int       `json:"deal_id"`       // ID сделки
	Currency      string    `json:"currency_id"`   // валюта (RUB/USD/...)

	StartPrincipalAmt   *float64 `json:"start_principal_amt"`   // первоначальная сумма (может быть пустой)
	CurrentPrincipalAmt float64  `json:"current_principal_amt"` // текущий остаток; если <0 -> депозит

	DealDate     time.Time `json:"deal_dttm"`        // дата выдачи сделки
	MaturityDate time.Time `json:"maturity_dttm"`    // дата погашения
	DealReportDt time.Time `json:"deal_report_dttm"` // дата отчёта по сделке

	RateType          string   `json:"rate_type_cd"`         // FIXED / FLOAT
	InterestRate      float64  `json:"interest_rate"`        // годовая ставка (0.075 = 7.5%)
	BaseIndicator     string   `json:"base_indicator"`       // имя индекса (CBKEY1M и т.п.)
	BaseIndicatorRate *float64 `json:"base_indicator_rate"`  // значение индекса (для FLOAT)
	DayConvention     string   `json:"day_convention_cd"`    // ACT/365 или ACT/ACT ISDA
	AccruedInterest   *float64 `json:"accrued_interest"`     // накопленный процент на report_dttm

	InterestPayStart  *time.Time `json:"interest_payment_rule_start_dttm"` // начало правила выплаты процентов
	InterestPayPeriod string     `json:"interest_payment_rule_period_cd"`  // периодичность выплаты процентов (1M/3M/...)
	InterestPayDayNum *int       `json:"interest_payment_rule_day_num"`    // день выплаты в месяце (anchor day)

	RegularPayStart  *time.Time `json:"regular_payment_rule_start_dttm"` // начало регулярных платежей тела
	RegularPayPeriod string     `json:"regular_payment_rule_period_cd"`  // периодичность платежей тела
	RegularPayDayNum *int       `json:"regular_payment_rule_day_num"`    // день платежа тела в месяце (anchor day)

	RepricingRuleStart    *time.Time `json:"repricing_rule_start_dttm"`            // начало правила переоценки ставки
	RepricingRulePeriod   string     `json:"repricing_rule_period_cd"`             // период переоценки
	RepricingRuleDayNum   *int       `json:"repricing_rule_day_num"`               // день переоценки
	RepricingDttmSchedule string     `json:"repricing_dttm_schedule"`              // список дат переоценки (строка)
	RepricingRateSchedule string     `json:"repricing_dttm_rate_schedule"`         // ставки по датам переоценки (строка)
	RepricingRateFixed    string     `json:"repricing_dttm_rate_schedule_fixed"`   // фиксированные ставки по датам (строка)

	Product         string `json:"product"`           // продукт 
	BehaviorModelCd string `json:"behavior_model_cd"` // модель досрочного погашения

	ManualInterestPayment  string `json:"interest_payment"`  // ручной график процентов
	ManualPrincipalPayment string `json:"principal_payment"` // ручной график тела: "2022-12-31,250000 2023-01-31,250000"

	CapLimitInterestRate   *float64 `json:"cap_limit_interest_rate"`   // верхняя граница ставки
	FloorLimitInterestRate *float64 `json:"floor_limit_interest_rate"` // нижняя граница ставки

	SubRateTypeCd             string     `json:"sub_rate_type_cd"`                    // под-ставка: тип (FIXED/FLOAT)
	SubInterestRate           *float64   `json:"sub_interest_rate"`                   // под-ставка: значение
	SubFloatIndicator         string     `json:"sub_float_indicator"`                 // под-ставка: индекс
	SubPaymentRulePeriodCd    string     `json:"sub_payment_rule_period_cd"`          // под-ставка: периодичность
	SubPaymentRuleDayNum      *int       `json:"sub_payment_rule_day_num"`            // под-ставка: день
	SubPaymentRuleStartDttm   *time.Time `json:"sub_payment_rule_start_dttm"`         // под-ставка: начало
	SubRepricingRuleStartDttm *time.Time `json:"sub_repricing_rule_start_dttm"`       // под-ставка: начало переоценки
	SubRepricingRulePeriodCd  string     `json:"sub_repricing_rule_period_cd"`        // под-ставка: период переоценки
	SubRepricingRuleDayNum    *int       `json:"sub_repricing_rule_day_num"`          // под-ставка: день переоценки
	SubCapLimitInterestRate   *float64   `json:"sub_cap_limit_interest_rate"`         // под-ставка: cap
	SubFloorLimitInterestRate *float64   `json:"sub_floor_limit_interest_rate"`       // под-ставка: floor
	SubCoeffIndex             *float64   `json:"sub_coeff_index"`                     // под-ставка: коэффициент
	SubRepricingFirstEventFlg *int       `json:"sub_repricing_rule_first_event_flg"`  // под-ставка: флаг первого события

	IndicatorLagCd    string `json:"indicator_lag_cd"`     // лаг индекса (напр. "-1M")
	SubIndicatorLagCd string `json:"sub_indicator_lag_cd"` // лаг под-индекса
}

type CashFlow struct {
	LoadDate   time.Time `json:"load_dttm"`      // когда посчитали (время запуска сервиса)
	SourceCode string    `json:"source_cd"`      // источник строки
	Discriminator string `json:"discriminator"`  // тип сделки
	DealID     int       `json:"deal_id"`        // ID сделки (для join с Deal)
	Currency   string    `json:"currency_cd"`    // валюта
	ReportDate    time.Time `json:"report_dttm"` // дата отчёта: от неё считаются платежи

	StartDate     time.Time `json:"start_dttm"`      // начало периода начисления (предыдущая дата платежа)
	PaymentDate   time.Time `json:"payment_dttm"`    // дата платежа (конец периода)
	DaysToPayment int       `json:"days_to_payment"` // дней от report_dttm до payment_dttm
	TimeBucket    string    `json:"time_bucket"`     // бакет (1d/1m/3m/.../30y)

	PaymentType string `json:"payment_type"` // тип строки: principal/interest/prepayment/close

	PrincipalAmount  float64 `json:"principal_amt"`    // сумма тела (со знаком: + кредит, − депозит)
	InterestAmount   float64 `json:"interest_amt"`     // сумма процентов (со знаком)
	RemainingBalance float64 `json:"remaining_balance"` // остаток после платежа (со знаком)
}
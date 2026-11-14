package cashflow

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

//одна точка истории индекса
type indexPoint struct {
	Date time.Time
	Rate float64
}

// отсортированная по дате история индекса
type indexSeries struct {
	Points []indexPoint
}

// ставка индекса на дату t: значение последней точки <= t
// Если t раньше первой точки - 0
// Если история пуста - false
func (s indexSeries) rateAt(t time.Time) (float64, bool) {
	if len(s.Points) == 0 {
		return 0, false
	}
	//бинарный поиск: первая точка с Date > t
	i := sort.Search(len(s.Points), func(i int) bool {
		return s.Points[i].Date.After(t)
	})
	if i == 0 {
		return 0, true
	}
	return s.Points[i-1].Rate, true
}

//читает файл "дата;ставка"
// Строки с ошибкой парсинга пропускаются (с логом через возвращаемую error-агрегацию).
func loadIndexSeries(path string) (indexSeries, error) {
	f, err := os.Open(path)
	if err != nil {
		return indexSeries{}, fmt.Errorf("loadIndexSeries: open %s: %w", path, err)
	}
	defer f.Close()

	var points []indexPoint
	sc := bufio.NewScanner(f)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ";", 2)
		if len(parts) != 2 {
			return indexSeries{}, fmt.Errorf("loadIndexSeries: %s:%d: expected 'date;rate', got %q", path, lineNo, line)
		}
		dt, err := parseDate(strings.TrimSpace(parts[0]))
		if err != nil {
			return indexSeries{}, fmt.Errorf("loadIndexSeries: %s:%d: date %q: %w", path, lineNo, parts[0], err)
		}
		rate, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if err != nil {
			return indexSeries{}, fmt.Errorf("loadIndexSeries: %s:%d: rate %q: %w", path, lineNo, parts[1], err)
		}
		points = append(points, indexPoint{Date: dt, Rate: rate})
	}
	if err := sc.Err(); err != nil {
		return indexSeries{}, fmt.Errorf("loadIndexSeries: read %s: %w", path, err)
	}

	sort.Slice(points, func(i, j int) bool {
		return points[i].Date.Before(points[j].Date)
	})
	return indexSeries{Points: points}, nil
}

//читает все известные индексы. Пока только CBKEY1M
// Возвращает map: имя индекса -> серия
// Отсутствие файла - не фатально: индекс просто не будет найден,
// и FLOAT-сделки с этим индексом упадут на базовую ставку
func loadIndices(dir string) map[string]indexSeries {
	out := make(map[string]indexSeries)

	cbkeyPath := dir + "/cbkey.txt"
	if s, err := loadIndexSeries(cbkeyPath); err == nil {
		out["CBKEY1M"] = s
	} else {
		// Логируем наверх через fmt - handler сам решит, печатать или нет
		// потом переделать на log, когда добавиться
		fmt.Printf("ВНИМАНИЕ: индекс CBKEY1M не загружен (%v); FLOAT-сделки с CBKEY1M будут использовать interest_rate\n", err)
	}

	return out
}
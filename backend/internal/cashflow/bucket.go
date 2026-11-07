package cashflow

import (
	
)

// распределяет интервал в днях по регуляторным корзинам IRRBB
func GetTimeBucket(days int) string {
	switch {
	case days <= 1:
		return "1d"
	case days <= 30:
		return "1m"
	case days <= 90:
		return "3m"
	case days <= 180:
		return "6m"
	case days <= 270:
		return "9m"
	case days <= 365:
		return "1y"
	case days <= 547:
		return "1.5y"
	case days <= 730:
		return "2y"
	case days <= 1095:
		return "3y"
	case days <= 1460:
		return "4y"
	case days <= 1825:
		return "5y"
	case days <= 2190:
		return "6y"
	case days <= 2555:
		return "7y"
	case days <= 2920:
		return "8y"
	case days <= 3285:
		return "9y"
	case days <= 3650:
		return "10y"
	case days <= 5475:
		return "15y"
	case days <= 7300:
		return "20y"
	default:
		return "30y"
	}
}
package cashflow

import "time"

type Deal struct {
	ReportDate    time.Time `json:"report_dttm"`
	Discriminator string    `json:"discriminator"`
	DealID        int       `json:"deal_id"`
	Currency      string    `json:"currency_id"`

	StartPrincipalAmt   *float64 `json:"start_principal_amt"`
	CurrentPrincipalAmt float64  `json:"current_principal_amt"`

	DealDate     time.Time `json:"deal_dttm"`
	MaturityDate time.Time `json:"maturity_dttm"`
	DealReportDt time.Time `json:"deal_report_dttm"`

	RateType          string   `json:"rate_type_cd"`
	InterestRate      float64  `json:"interest_rate"`
	BaseIndicator     string   `json:"base_indicator"`
	BaseIndicatorRate *float64 `json:"base_indicator_rate"`
	DayConvention     string   `json:"day_convention_cd"`
	AccruedInterest   *float64 `json:"accrued_interest"`

	InterestPayStart  *time.Time `json:"interest_payment_rule_start_dttm"`
	InterestPayPeriod string     `json:"interest_payment_rule_period_cd"`
	InterestPayDayNum *int       `json:"interest_payment_rule_day_num"`

	RegularPayStart  *time.Time `json:"regular_payment_rule_start_dttm"`
	RegularPayPeriod string     `json:"regular_payment_rule_period_cd"`
	RegularPayDayNum *int       `json:"regular_payment_rule_day_num"`

	RepricingRuleStart    *time.Time `json:"repricing_rule_start_dttm"`
	RepricingRulePeriod   string     `json:"repricing_rule_period_cd"`
	RepricingRuleDayNum   *int       `json:"repricing_rule_day_num"`
	RepricingDttmSchedule string     `json:"repricing_dttm_schedule"`
	RepricingRateSchedule string     `json:"repricing_dttm_rate_schedule"`
	RepricingRateFixed    string     `json:"repricing_dttm_rate_schedule_fixed"`

	Product         string `json:"product"`
	BehaviorModelCd string `json:"behavior_model_cd"`

	ManualInterestPayment  string `json:"interest_payment"`
	ManualPrincipalPayment string `json:"principal_payment"`

	CapLimitInterestRate   *float64 `json:"cap_limit_interest_rate"`
	FloorLimitInterestRate *float64 `json:"floor_limit_interest_rate"`

	SubRateTypeCd             string     `json:"sub_rate_type_cd"`
	SubInterestRate           *float64   `json:"sub_interest_rate"`
	SubFloatIndicator         string     `json:"sub_float_indicator"`
	SubPaymentRulePeriodCd    string     `json:"sub_payment_rule_period_cd"`
	SubPaymentRuleDayNum      *int       `json:"sub_payment_rule_day_num"`
	SubPaymentRuleStartDttm   *time.Time `json:"sub_payment_rule_start_dttm"`
	SubRepricingRuleStartDttm *time.Time `json:"sub_repricing_rule_start_dttm"`
	SubRepricingRulePeriodCd  string     `json:"sub_repricing_rule_period_cd"`
	SubRepricingRuleDayNum    *int       `json:"sub_repricing_rule_day_num"`
	SubCapLimitInterestRate   *float64   `json:"sub_cap_limit_interest_rate"`
	SubFloorLimitInterestRate *float64   `json:"sub_floor_limit_interest_rate"`
	SubCoeffIndex             *float64   `json:"sub_coeff_index"`
	SubRepricingFirstEventFlg *int       `json:"sub_repricing_rule_first_event_flg"`

	IndicatorLagCd    string `json:"indicator_lag_cd"`
	SubIndicatorLagCd string `json:"sub_indicator_lag_cd"`
}

const (
	TypeInterestPayment  = "interest_payment"
	TypePrincipalPayment = "principal_payment"
	TypePrepayment       = "prepayment"
	TypeClosePayment     = "close_payment"
	TypeRepricing        = "repricing"
	TypeTimeBucketEnd    = "time_bucket_end"
)

const (
	DiscANN    = "ANN"
	DiscRGM    = "RGM"
	DiscSIMPLE = "SIMPLE"
)

type CashFlow struct {
	LoadDate   time.Time `json:"load_dttm"`
	SourceCode string    `json:"source_cd"`
	DealID     int       `json:"deal_id"`
	Currency   string    `json:"currency_cd"`

	StartDate     time.Time `json:"start_dttm"`
	PaymentDate   time.Time `json:"payment_dttm"`
	DaysToPayment int       `json:"days_to_payment"`
	TimeBucket    string    `json:"time_bucket"`

	PaymentType string `json:"payment_type"`

	PrincipalAmount  float64 `json:"principal_amt"`
	InterestAmount   float64 `json:"interest_amt"`
	RemainingBalance float64 `json:"remaining_balance"`
}
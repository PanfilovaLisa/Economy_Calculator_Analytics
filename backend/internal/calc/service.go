package calc

import "analytics/configs"

type CalcServiceDeps struct {
	Config *configs.Config
}

type CalcService struct {
	Config *configs.Config
}

func NewCalcService(deps CalcServiceDeps) *CalcService {
	return &CalcService{
		Config: deps.Config,
	}
}
package service

import "context"

//go:generate $MOCKGEN -source=contract_checker.go -destination=mocks/contract_checker.gen.go -package=mocks

type ContractChecker interface {
	CheckOption(ctx context.Context, companyID string, optionCode string) (CheckResult, error)
}

type CheckResult struct {
	Allowed bool
	Reason  string
}

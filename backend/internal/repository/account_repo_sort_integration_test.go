//go:build integration

package repository

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (s *AccountRepoSuite) TestList_DefaultSortByNameAsc() {
	mustCreateAccount(s.T(), s.client, &service.Account{Name: "z-account"})
	mustCreateAccount(s.T(), s.client, &service.Account{Name: "a-account"})

	accounts, _, err := s.repo.List(s.ctx, pagination.PaginationParams{Page: 1, PageSize: 10})
	s.Require().NoError(err)
	s.Require().Len(accounts, 2)
	s.Require().Equal("a-account", accounts[0].Name)
	s.Require().Equal("z-account", accounts[1].Name)
}

func (s *AccountRepoSuite) TestListWithFilters_SortByPriorityDesc() {
	mustCreateAccount(s.T(), s.client, &service.Account{Name: "low-priority", Priority: 10})
	mustCreateAccount(s.T(), s.client, &service.Account{Name: "high-priority", Priority: 90})

	accounts, _, err := s.repo.ListWithFilters(s.ctx, pagination.PaginationParams{
		Page:      1,
		PageSize:  10,
		SortBy:    "priority",
		SortOrder: "desc",
	}, "", "", "", "", 0, "")
	s.Require().NoError(err)
	s.Require().Len(accounts, 2)
	s.Require().Equal("high-priority", accounts[0].Name)
	s.Require().Equal("low-priority", accounts[1].Name)
}

func (s *AccountRepoSuite) TestListWithFilters_SortByStatusRecoveryAtAsc() {
	now := time.Now().UTC()
	past := now.Add(-10 * time.Minute)
	overload := now.Add(10 * time.Minute)
	tempUnsched := now.Add(20 * time.Minute)
	rateLimit := now.Add(30 * time.Minute)

	mustCreateAccount(s.T(), s.client, &service.Account{Name: "no-recovery"})
	mustCreateAccount(s.T(), s.client, &service.Account{Name: "past-rate-limit", RateLimitResetAt: &past})
	mustCreateAccount(s.T(), s.client, &service.Account{Name: "rate-limit-30m", RateLimitResetAt: &rateLimit})
	mustCreateAccount(s.T(), s.client, &service.Account{Name: "overload-10m", OverloadUntil: &overload})
	tempAccount := mustCreateAccount(s.T(), s.client, &service.Account{Name: "temp-unsched-20m"})
	s.Require().NoError(s.repo.SetTempUnschedulable(s.ctx, tempAccount.ID, tempUnsched, "test"))

	accounts, _, err := s.repo.ListWithFilters(s.ctx, pagination.PaginationParams{
		Page:      1,
		PageSize:  10,
		SortBy:    "status_recovery_at",
		SortOrder: "asc",
	}, "", "", "", "", 0, "")
	s.Require().NoError(err)
	s.Require().Len(accounts, 5)
	s.Require().Equal("overload-10m", accounts[0].Name)
	s.Require().Equal("temp-unsched-20m", accounts[1].Name)
	s.Require().Equal("rate-limit-30m", accounts[2].Name)
	s.Require().Equal("no-recovery", accounts[3].Name)
	s.Require().Equal("past-rate-limit", accounts[4].Name)
}

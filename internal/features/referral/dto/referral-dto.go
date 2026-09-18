package dto

import "time"

type (
	SeeWhoseReferralAlreadyRequest struct {
		Username string
	}
	SeeWhoseReferralAlreadyResponse struct {
		Referrer      string
		ReferrerURL   string
		ReferralSince time.Time
		Message       string
	}
	ContestBetweenReferralsRequest struct {
	}
	ContestBetweenReferralsResponse struct {
	}
	CurrentReferralListRequest struct {
	}
	CurrentReferralListResponse struct {
	}
)

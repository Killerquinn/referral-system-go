package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

type ReferralService interface {
}

type ReferralHandler struct {
	rService ReferralService
}

func NewRefferalHandler(rs ReferralService) *ReferralHandler {
	return &ReferralHandler{rService: rs}
}

func Register(r chi.Router, rs ReferralService) {
	rh := NewRefferalHandler(rs)

	r.Get("api/v1/referrals", rh.CurrentReferralList)
	r.Post("api/v1/referrals/draw", rh.ContestBetweenReferrals)
	r.Post("api/v1/referrals/becomereff", rh.BecomeSomeonesReferral)
	r.Get("api/v1/refferer", rh.SeeWhoseReferralAlready)
}

func (rh *ReferralHandler) CurrentReferralList(w http.ResponseWriter, r *http.Request) {
	panic("implement me!")
}

func (rh *ReferralHandler) ContestBetweenReferrals(w http.ResponseWriter, r *http.Request) {
	panic("implement me!")
}

func (rh *ReferralHandler) BecomeSomeonesReferral(w http.ResponseWriter, r *http.Request) {
	panic("implement me!")
}

func (rh *ReferralHandler) SeeWhoseReferralAlready(w http.ResponseWriter, r *http.Request) {
	panic("implement me!")
}

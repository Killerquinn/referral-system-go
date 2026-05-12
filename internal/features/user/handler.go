package user

import "net/http"

type UseCase interface {
}

type Service struct {
	useCase UseCase
}

func NewHandler() *Service {
	return &Service{useCase: useCase}
}

func (h *Service) CreateUser(w http.ResponseWriter, r *http.Request) {}

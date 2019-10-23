package user

import (
	"net/http"
)

type Registry interface {
	User (id int) (interface{}, bool)
	Login (w http.ResponseWriter, r *http.Request) (id int, user interface {}, e error)
}

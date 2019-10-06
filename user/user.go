package user

type Registry interface {
	User (id int) (interface{}, bool)
}

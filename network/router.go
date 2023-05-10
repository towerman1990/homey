package network

type (
	Router interface {
		// hook function
		PreHandle(request Request) error

		Handle(request Request) error

		PostHandle(request Request) error
	}

	BaseRouter struct{}
)

func (br *BaseRouter) PreHandle(request Request) (err error) {
	return
}

func (br *BaseRouter) Handle(request Request) (err error) {
	return
}

func (br *BaseRouter) PostHandle(request Request) (err error) {
	return
}

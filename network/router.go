package network

type (
	Router interface {
		PreHandle(request Request)

		Handle(request Request)

		PostHandle(request Request)
	}

	BaseRouter struct{}
)

func (br *BaseRouter) PreHandle(request Request) {}

func (br *BaseRouter) Handle(request Request) {}

func (br *BaseRouter) PostHandle(request Request) {}

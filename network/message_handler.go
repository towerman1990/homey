package network

import (
	"fmt"

	"github.com/homey/config"
	"github.com/labstack/gommon/log"
)

type MessageHandler interface {
	ExecHandler(request Request)

	AddRouter(msgID uint32, router Router) error

	StartWorkPool()

	SendMsgToTaskQueue(Request)
}

type messageHandler struct {
	Handlers map[uint32]Router

	TaskQueue []chan Request

	WorkerPoolSize uint32
}

func (mh *messageHandler) ExecHandler(request Request) {
	packageType := request.GetMessagePackageType()
	handler, ok := mh.Handlers[packageType]
	if !ok {
		fmt.Printf("packageType [%d] hasn't been added\n", packageType)
		return
	}

	handler.PreHandle(request)
	handler.Handle(request)
	handler.PostHandle(request)
}

func (mh *messageHandler) AddRouter(packageType uint32, router Router) (err error) {
	if _, ok := mh.Handlers[packageType]; ok {
		return fmt.Errorf("packageType [%d] has been added", packageType)
	}

	mh.Handlers[packageType] = router
	log.Printf("added router successfully, packageType = [%d]", packageType)

	return
}

func (mh *messageHandler) StartWorkPool() {
	for i := 0; i < int(mh.WorkerPoolSize); i++ {
		mh.TaskQueue[i] = make(chan Request, config.GlobalConfig.MaxWorkerTaskLen)
		go mh.StartOneWork(i)
	}
}

func (mh *messageHandler) StartOneWork(i int) {
	log.Printf("worker [%d] started", i)

	for request := range mh.TaskQueue[i] {
		mh.ExecHandler(request)
	}
}

func (mh *messageHandler) SendMsgToTaskQueue(request Request) {
	workerID := request.GetConnection().GetID() % uint64(mh.WorkerPoolSize)
	mh.TaskQueue[workerID] <- request
}

func NewMessageHandler() MessageHandler {
	return &messageHandler{
		Handlers:       make(map[uint32]Router),
		WorkerPoolSize: config.GlobalConfig.WorkerPoolSize,
		TaskQueue:      make([]chan Request, config.GlobalConfig.MaxWorkerTaskLen),
	}
}

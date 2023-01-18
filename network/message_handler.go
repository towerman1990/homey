package network

import (
	"fmt"

	"github.com/homey/config"
	log "github.com/homey/logger"
	"go.uber.org/zap"
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
	dataType := request.GetMessageDataType()
	handler, ok := mh.Handlers[dataType]
	if !ok {
		log.Logger.Warn("data type hasn't been added to router", zap.Uint32("data_type", dataType))

		return
	}

	handler.PreHandle(request)
	handler.Handle(request)
	handler.PostHandle(request)
}

func (mh *messageHandler) AddRouter(dataType uint32, router Router) (err error) {
	if _, ok := mh.Handlers[dataType]; ok {
		return fmt.Errorf("the data type [%d] has been added", dataType)
	}

	mh.Handlers[dataType] = router
	log.Logger.Info("added router successfully", zap.Uint32("data_type", dataType))

	return
}

func (mh *messageHandler) StartWorkPool() {
	for i := 0; i < int(mh.WorkerPoolSize); i++ {
		mh.TaskQueue[i] = make(chan Request, config.GlobalConfig.MaxWorkerTaskLen)
		go mh.StartOneWork(i)
	}
}

func (mh *messageHandler) StartOneWork(i int) {
	log.Logger.Info("new worker started", zap.Int("worker_id", i))

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

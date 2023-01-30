package network

import (
	"fmt"

	"github.com/towerman1990/homey/config"
	log "github.com/towerman1990/homey/logger"
	"go.uber.org/zap"
)

type (
	MessageHandler interface {
		// execute handler function
		ExecHandler(request Request)

		// add router
		AddRouter(msgType uint32, router Router)

		// start work pool
		StartWorkPool()

		// send message to task queue, the message would be handled by worker
		SendMsgToTaskQueue(Request)

		String()
	}

	messageHandler struct {
		Handlers map[uint32]Router

		TaskQueue []chan Request

		WorkerPoolSize uint32
	}
)

func (mh *messageHandler) ExecHandler(request Request) {
	dataType := request.GetMsgDataType()
	handler, ok := mh.Handlers[dataType]
	if !ok {
		log.Logger.Warn("data type hasn't been bound on handler", zap.Uint32("dataType", dataType))
		return
	}

	if err := handler.PreHandle(request); err != nil {
		log.Logger.Error("failed to execute PreHandle function", zap.String("error", err.Error()))
		return
	}

	if err := handler.Handle(request); err != nil {
		log.Logger.Error("failed to execute PreHandle function", zap.String("error", err.Error()))
		return
	}

	if err := handler.PostHandle(request); err != nil {
		log.Logger.Error("failed to execute PreHandle function", zap.String("error", err.Error()))
		return
	}
}

func (mh *messageHandler) AddRouter(dataType uint32, router Router) {
	if _, ok := mh.Handlers[dataType]; ok {
		log.Logger.Error("the data type has been added", zap.Uint32("dataType", dataType))
	}

	mh.Handlers[dataType] = router
	log.Logger.Info("added router successfully", zap.Uint32("dataType", dataType))
}

func (mh *messageHandler) StartWorkPool() {
	for i := 0; i < int(mh.WorkerPoolSize); i++ {
		mh.TaskQueue[i] = make(chan Request, config.Global.MaxWorkerTaskLen)
		go mh.StartOneWork(i)
	}
}

func (mh *messageHandler) StartOneWork(i int) {
	log.Logger.Info("new worker started", zap.Int("workerID", i))

	for request := range mh.TaskQueue[i] {
		mh.ExecHandler(request)
	}
}

func (mh *messageHandler) SendMsgToTaskQueue(request Request) {
	workerID := request.GetConnection().GetID() % uint64(mh.WorkerPoolSize)
	mh.TaskQueue[workerID] <- request
}

func (mh *messageHandler) String() {
	fmt.Printf("mh.Handlers: %v\n", mh.Handlers)
}

func NewMessageHandler() MessageHandler {
	return &messageHandler{
		Handlers:       make(map[uint32]Router),
		WorkerPoolSize: config.Global.WorkerPoolSize,
		TaskQueue:      make([]chan Request, config.Global.MaxWorkerTaskLen),
	}
}

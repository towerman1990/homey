package network

import (
	"context"
	"fmt"
	"sync"

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
		queueCapacity  uint32
		ctx            context.Context
		routesMu       sync.RWMutex
		startOnce      sync.Once
		startTask      func(func())
	}
)

func (mh *messageHandler) ExecHandler(request Request) {
	defer func() {
		if recovered := recover(); recovered != nil {
			log.Logger.Error("message handler panicked", zap.Any("panic", recovered), zap.Stack("stack"))
		}
	}()
	dataType := request.GetMsgDataType()
	mh.routesMu.RLock()
	handler, ok := mh.Handlers[dataType]
	mh.routesMu.RUnlock()
	if !ok {
		log.Logger.Warn("data type hasn't been bound on handler", zap.Uint32("dataType", dataType))
		return
	}

	if err := handler.PreHandle(request); err != nil {
		log.Logger.Error("failed to execute PreHandle function", zap.String("error", err.Error()))
		return
	}

	if err := handler.Handle(request); err != nil {
		log.Logger.Error("failed to execute Handle function", zap.String("error", err.Error()))
		return
	}

	if err := handler.PostHandle(request); err != nil {
		log.Logger.Error("failed to execute PostHandle function", zap.String("error", err.Error()))
		return
	}
}

func (mh *messageHandler) AddRouter(dataType uint32, router Router) {
	mh.routesMu.Lock()
	defer mh.routesMu.Unlock()
	if _, ok := mh.Handlers[dataType]; ok {
		log.Logger.Error("the data type has been added", zap.Uint32("dataType", dataType))
		return
	}

	mh.Handlers[dataType] = router
	log.Logger.Info("added router successfully", zap.Uint32("dataType", dataType))
}

func (mh *messageHandler) StartWorkPool() {
	mh.startOnce.Do(func() {
		for i := range mh.TaskQueue {
			mh.TaskQueue[i] = make(chan Request, mh.queueCapacity)
			queue := mh.TaskQueue[i]
			if mh.startTask != nil {
				mh.startTask(func() { mh.startWorker(queue) })
			} else {
				go mh.startWorker(queue)
			}
		}
	})
}

func (mh *messageHandler) startWorker(queue <-chan Request) {
	for mh.ctx.Err() == nil {
		select {
		case <-mh.ctx.Done():
			return
		case req := <-queue:
			if mh.ctx.Err() == nil && req.GetConnection().Context().Err() == nil {
				mh.ExecHandler(req)
			}
		}
	}
}

func (mh *messageHandler) SendMsgToTaskQueue(req Request) {
	mh.enqueue(req)
}

func (mh *messageHandler) enqueue(req Request) bool {
	conn := req.GetConnection()
	if conn.Context().Err() != nil || mh.ctx.Err() != nil {
		return false
	}
	if mh.WorkerPoolSize == 0 {
		mh.ExecHandler(req)
		return true
	}

	mh.StartWorkPool()
	workerID := conn.GetID() % uint64(mh.WorkerPoolSize)
	if mh.queueCapacity == 0 {
		select {
		case <-conn.Context().Done():
			return false
		case <-mh.ctx.Done():
			return false
		case mh.TaskQueue[workerID] <- req:
			return true
		}
	}
	select {
	case mh.TaskQueue[workerID] <- req:
		return true
	default:
		log.Logger.Warn("worker queue full", zap.Uint64("connection", conn.GetID()))
		conn.Close()
		return false
	}
}

func (mh *messageHandler) String() {
	mh.routesMu.RLock()
	defer mh.routesMu.RUnlock()
	fmt.Printf("mh.Handlers: %v\n", mh.Handlers)
}

func NewMessageHandler() MessageHandler {
	return newMessageHandler(context.Background(), config.Global)
}

func newMessageHandler(ctx context.Context, cfg config.GlobalConfig) *messageHandler {
	return &messageHandler{
		Handlers:       make(map[uint32]Router),
		WorkerPoolSize: cfg.WorkerPoolSize,
		TaskQueue:      make([]chan Request, cfg.WorkerPoolSize),
		queueCapacity:  cfg.MaxWorkerTaskLen,
		ctx:            ctx,
	}
}

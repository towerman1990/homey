package network

type Request interface {

	// get request belong connection
	GetConnection() Connection

	// get message data type
	GetMessageDataType() uint32

	// get message data
	GetMessageData() []byte
}

type request struct {

	// request belong connection
	conn Connection

	// request contains message
	msg Message
}

func (r *request) GetConnection() Connection {
	return r.conn
}

func (r *request) GetMessageData() []byte {
	return r.msg.GetData()
}

func (r *request) GetMessageDataType() uint32 {
	return r.msg.GetDataType()
}

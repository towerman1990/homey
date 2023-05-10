package network

type (
	Request interface {

		// get request belong connection
		GetConnection() Connection

		// get custom message type which was bound on handler
		GetMsgDataType() uint32

		// get message data
		GetMsgData() []byte
	}

	request struct {

		// request belong connection
		conn Connection

		// request contains message
		msg Message
	}
)

func (r *request) GetConnection() Connection {
	return r.conn
}

func (r *request) GetMsgData() []byte {
	return r.msg.GetData()
}

func (r *request) GetMsgDataType() uint32 {
	return r.msg.GetDataType()
}

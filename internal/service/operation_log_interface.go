package service

type OperationLogService interface {
	Log(username, actionType, object, description string, success bool) error
}

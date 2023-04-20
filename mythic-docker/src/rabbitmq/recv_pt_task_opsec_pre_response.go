package rabbitmq

import (
	"encoding/json"

	"github.com/its-a-feature/Mythic/database"
	databaseStructs "github.com/its-a-feature/Mythic/database/structs"
	"github.com/its-a-feature/Mythic/logging"
	amqp "github.com/rabbitmq/amqp091-go"
)

func init() {
	RabbitMQConnection.AddDirectQueue(DirectQueueStruct{
		Exchange:   MYTHIC_EXCHANGE,
		Queue:      PT_TASK_OPSEC_PRE_CHECK_RESPONSE,
		RoutingKey: PT_TASK_OPSEC_PRE_CHECK_RESPONSE,
		Handler:    processPtTaskOPSECPreMessages,
	})
}

func processPtTaskOPSECPreMessages(msg amqp.Delivery) {
	payloadMsg := PTTTaskOPSECPreTaskMessageResponse{}
	if err := json.Unmarshal(msg.Body, &payloadMsg); err != nil {
		logging.LogError(err, "Failed to process message into struct")
	} else {
		task := databaseStructs.Task{}
		task.ID = payloadMsg.TaskID
		if task.ID <= 0 {
			// we ran into an error and couldn't even get the task information out
			go SendAllOperationsMessage(payloadMsg.Error, 0, "", database.MESSAGE_LEVEL_WARNING)
			return
		}
		if payloadMsg.Success {
			shouldMoveToCreateTasking := false
			if !payloadMsg.OpsecPreBlocked || (payloadMsg.OpsecPreBlocked && payloadMsg.OpsecPreBypassed != nil && *payloadMsg.OpsecPreBypassed) {
				shouldMoveToCreateTasking = true
				task.Status = PT_TASK_FUNCTION_STATUS_PREPROCESSING
			} else {
				task.Status = PT_TASK_FUNCTION_STATUS_OPSEC_PRE_BLOCKED
			}

			task.OpsecPreBlocked.Bool = payloadMsg.OpsecPreBlocked
			task.OpsecPreBlocked.Valid = true
			task.OpsecPreBypassRole = string(payloadMsg.OpsecPreBypassRole)
			if payloadMsg.OpsecPreBypassed != nil {
				task.OpsecPreBypassed = *payloadMsg.OpsecPreBypassed
			} else if payloadMsg.OpsecPreBlocked {
				task.OpsecPreBypassed = false
			} else {
				task.OpsecPreBypassed = true
			}
			task.OpsecPreMessage = payloadMsg.OpsecPreMessage
			if _, err := database.DB.NamedExec(`UPDATE task SET
			status=:status, opsec_pre_blocked=:opsec_pre_blocked, opsec_pre_bypass_role=:opsec_pre_bypass_role,
			opsec_pre_bypassed=:opsec_pre_bypassed, opsec_pre_message=:opsec_pre_message 
			WHERE id=:id`, task); err != nil {
				logging.LogError(err, "Failed to update task status")
				return
			} else if shouldMoveToCreateTasking {
				allTaskData := GetTaskConfigurationForContainer(task.ID)
				if err := RabbitMQConnection.SendPtTaskCreate(allTaskData); err != nil {
					logging.LogError(err, "In processPtTaskOPSECPreMessages, but failed to sendSendPtTaskCreate ")
				}
				return
			}
		} else {
			task.Status = PT_TASK_FUNCTION_STATUS_OPSEC_PRE_ERROR
			logging.LogInfo("response", "task", payloadMsg)
			task.Stderr = payloadMsg.Error
			if _, err := database.DB.NamedExec(`UPDATE task SET
			status=:status, stderr=:stderr 
			WHERE
			id=:id`, task); err != nil {
				logging.LogError(err, "Failed to update task status")
				return
			}
		}
	}
}

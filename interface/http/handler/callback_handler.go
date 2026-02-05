package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/alexmorbo/keep-mattermost-bridge/application/dto"
	"github.com/alexmorbo/keep-mattermost-bridge/application/port"
)

type CallbackHandlerHTTP struct {
	handleCallback port.CallbackUseCase
}

func NewCallbackHandler(handleCallback port.CallbackUseCase) *CallbackHandlerHTTP {
	return &CallbackHandlerHTTP{handleCallback: handleCallback}
}

func (h *CallbackHandlerHTTP) HandleCallback(c *gin.Context) {
	var input dto.MattermostCallbackInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	result, err := h.handleCallback.ExecuteImmediate(input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	h.handleCallback.ExecuteAsync(input)

	response := gin.H{
		"update": gin.H{
			"message": "",
			"props": gin.H{
				"attachments": []gin.H{attachmentToJSON(result.Attachment)},
			},
		},
	}

	c.JSON(http.StatusOK, response)
}

func attachmentToJSON(a dto.AttachmentDTO) gin.H {
	fields := make([]gin.H, len(a.Fields))
	for i, f := range a.Fields {
		fields[i] = gin.H{"title": f.Title, "value": f.Value, "short": f.Short}
	}

	actions := make([]gin.H, len(a.Actions))
	for i, b := range a.Actions {
		action := gin.H{
			"id":   b.ID,
			"name": b.Name,
			"integration": gin.H{
				"url":     b.Integration.URL,
				"context": b.Integration.Context,
			},
		}
		if b.Style != "" {
			action["style"] = b.Style
		}
		actions[i] = action
	}

	result := gin.H{
		"color":       a.Color,
		"title":       a.Title,
		"title_link":  a.TitleLink,
		"fields":      fields,
		"actions":     actions,
		"footer":      a.Footer,
		"footer_icon": a.FooterIcon,
	}

	if a.Text != "" {
		result["text"] = a.Text
	}

	return result
}

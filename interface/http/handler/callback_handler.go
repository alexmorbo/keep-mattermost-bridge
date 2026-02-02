package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/alexmorbo/keep-mattermost-bridge/application/dto"
)

type CallbackHandler interface {
	Execute(ctx context.Context, input dto.MattermostCallbackInput) (*dto.CallbackOutput, error)
}

type CallbackHandlerHTTP struct {
	handleCallback CallbackHandler
}

func NewCallbackHandler(handleCallback CallbackHandler) *CallbackHandlerHTTP {
	return &CallbackHandlerHTTP{handleCallback: handleCallback}
}

func (h *CallbackHandlerHTTP) HandleCallback(c *gin.Context) {
	var input dto.MattermostCallbackInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	result, err := h.handleCallback.Execute(ctx, input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	response := gin.H{
		"update": gin.H{
			"message": "",
			"props": gin.H{
				"attachments": []gin.H{attachmentToJSON(result.Attachment)},
			},
		},
		"ephemeral_text": result.Ephemeral,
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
		actions[i] = gin.H{
			"id":   b.ID,
			"name": b.Name,
			"integration": gin.H{
				"url":     b.Integration.URL,
				"context": b.Integration.Context,
			},
		}
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

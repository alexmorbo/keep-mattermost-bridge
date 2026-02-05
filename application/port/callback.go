package port

import "github.com/alexmorbo/keep-mattermost-bridge/application/dto"

type CallbackUseCase interface {
	ExecuteImmediate(input dto.MattermostCallbackInput) (*dto.CallbackOutput, error)
	ExecuteAsync(input dto.MattermostCallbackInput)
}

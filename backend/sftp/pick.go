package sftp

import (
	"context"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// PickUploadFiles 原生对话框多选文件（上传文件…）。
func (s *Service) PickUploadFiles(ctx context.Context) ([]string, error) {
	d := application.Get().Dialog.OpenFile()
	d.CanChooseFiles(true).CanChooseDirectories(false).SetTitle("选择要上传的文件")
	return d.PromptForMultipleSelection()
}

// PickUploadFolder 原生对话框选择文件夹（上传文件夹…）。
func (s *Service) PickUploadFolder(ctx context.Context) (string, error) {
	d := application.Get().Dialog.OpenFile()
	d.CanChooseDirectories(true).CanChooseFiles(false).SetTitle("选择要上传的文件夹")
	return d.PromptForSingleSelection()
}

// PickDownloadFolder 原生对话框选择本地目标文件夹（下载到…）。
func (s *Service) PickDownloadFolder(ctx context.Context) (string, error) {
	d := application.Get().Dialog.OpenFile()
	d.CanChooseDirectories(true).CanChooseFiles(false).SetTitle("选择下载目标文件夹")
	return d.PromptForSingleSelection()
}
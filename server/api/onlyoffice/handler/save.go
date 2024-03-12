/**
 *
 * (c) Copyright Ascensio System SIA 2023
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */
package handler

import (
	"fmt"
	"github.com/ONLYOFFICE/onlyoffice-mattermost/server/api"
	"github.com/ONLYOFFICE/onlyoffice-mattermost/server/api/onlyoffice/model"
	"github.com/mattermost/mattermost-server/v6/shared/filestore"
	"github.com/pkg/errors"
	"net/http"
	"reflect"
)

var _ = Registry.RegisterHandler(2, _saveFile)
var _ = Registry.RegisterHandler(6, _saveFile)

func _saveFile(c model.Callback, a api.PluginAPI) error {
	a.API.LogDebug(_OnlyofficeLoggerPrefix + "file " + c.FileID + " save call")

	if c.URL == "" {
		return &InvalidFileDownloadUrlError{
			FileID: c.FileID,
		}
	}

	fileInfo, fileErr := a.API.GetFileInfo(c.FileID)
	if fileErr != nil {
		return &FileNotFoundError{
			FileID: c.FileID,
			Reason: fileErr.Error(),
		}
	}

	resp, err := http.Get(c.URL)
	if resp != nil {
		defer resp.Body.Close()
	}

	if err != nil {
		return errors.Wrap(err, _OnlyofficeLoggerPrefix)
	}

	post, postErr := a.API.GetPost(fileInfo.PostId)
	if postErr != nil {
		return &FilePersistenceError{
			FileID: c.FileID,
			Reason: postErr.Error(),
		}
	}

	fileSettings := a.API.GetConfig().FileSettings

	debugMsg := fmt.Sprintf("文件路径 %s  minio路径: %s ssl是否打开: %v", fileInfo.Path, *fileSettings.AmazonS3Endpoint, fileSettings.AmazonS3SSL)
	// FIXME debug message
	a.Bot.BotCreateReply(debugMsg, post.ChannelId, post.Id)

	printfStore(a.Filestore, func(message string) {
		a.Bot.BotCreateReply(message, post.ChannelId, post.Id)
	})

	connectError := a.Filestore.TestConnection()
	debugMsg = fmt.Sprintf("新建测试连接报错 %s", connectError)
	// FIXME debug message
	a.Bot.BotCreateReply(debugMsg, post.ChannelId, post.Id)
	post.UpdateAt = a.OnlyofficeConverter.GetTimestamp()
	_, uErr := a.API.UpdatePost(post)
	if uErr != nil {
		return &FilePersistenceError{
			FileID: c.FileID,
			Reason: uErr.Error(),
		}
	}

	_, storeErr := a.Filestore.WriteFile(resp.Body, fileInfo.Path)
	if storeErr != nil {
		debugMsg = fmt.Sprintf("保存文件失败 %s", storeErr)
		// FIXME debug message
		a.Bot.BotCreateReply(debugMsg, post.ChannelId, post.Id)
		return &FilePersistenceError{
			FileID: c.FileID,
			Reason: storeErr.Error(),
		}
	}

	if c.Status == 2 {
		last := c.Users[0]
		if last == "" {
			return ErrInvalidUserID
		}

		user, userErr := a.API.GetUser(last)
		if userErr != nil {
			return &FilePersistenceError{
				FileID: c.FileID,
				Reason: userErr.Error(),
			}
		}

		replyMsg := fmt.Sprintf("File %s was updated by @%s", fileInfo.Name, user.Username)
		a.Bot.BotCreateReply(replyMsg, post.ChannelId, post.Id)
	}

	return nil
}

func printfStore(fileStore filestore.FileBackend, f func(message string)) {
	v := reflect.ValueOf(fileStore)
	endpoint := v.FieldByName("endpoint")
	secure := v.FieldByName("secure")
	prefix := v.FieldByName("pathPrefix")
	msg := fmt.Sprintf("目前端点:%v 安全:%v 前缀:%v ", endpoint, secure, prefix)
	f(msg)
}

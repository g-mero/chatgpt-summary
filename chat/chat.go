package chat

import (
	"bufio"
	"errors"
	"github.com/duke-git/lancet/v2/random"
	"github.com/imroc/req/v3"
	"github.com/tidwall/gjson"
	"io"
	"strconv"
	"strings"
)

type Gpt struct {
	token  string
	proxy  string
	client *req.Client
}

func InitGpt(token string, proxy ...string) Gpt {
	thisProxy := "https://ai.fakeopen.com/api/conversation"
	if len(proxy) > 0 {
		thisProxy = proxy[0]
	}

	client := req.C().SetCommonHeaders(map[string]string{
		"accept":          "text/event-stream",
		"accept-language": "de-DE,de;q=0.9,en-US;q=0.8,en;q=0.7",
		"user-agent":      "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/100.0.4896.75 Safari/537.36",
	}).DisableAutoReadResponse()

	return Gpt{
		token:  token,
		proxy:  thisProxy,
		client: client,
	}
}

func (that Gpt) conversationSSE(jsonBody string) (io.ReadCloser, error) {
	resp, err := that.client.R().SetBodyJsonString(jsonBody).SetBearerAuthToken(that.token).Post(that.proxy)

	if err != nil {
		return nil, err
	}

	if resp.IsSuccessState() {
		return resp.Body, nil
	}

	return nil, errors.New("请求失败： " + resp.Status)
}

func (that Gpt) SingleConversation(question string) (string, error) {
	body, err := that.conversationSSE(makeBody(question))

	if err != nil {
		return "", err
	}

	defer func(body io.ReadCloser) {
		_ = body.Close()
	}(body)

	// 读取sse时间流，获取完整数据
	scanner := bufio.NewScanner(body)
	var result string
	for scanner.Scan() {
		eventData := scanner.Text()

		// 检查是否是一个完整的事件（通常以 "data: " 开头）
		if strings.HasPrefix(eventData, "data: ") {
			eventData = strings.TrimPrefix(eventData, "data: ")
			status := gjson.Get(eventData, "message.metadata.is_complete")
			if status.String() == "true" {
				res := gjson.Get(eventData, "message.content.parts.0")
				result = res.String()
				break
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return result, nil
}

func (that Gpt) MakeSummary(content string) (string, error) {
	prompt := "生成这篇文章的中文摘要，字数不能超过150个汉字，要求以\"这篇文章讲了\"开头，尽可能简短，直接输出结果\n"
	result, err := that.SingleConversation(prompt + content)
	if err != nil {
		return "", err
	}

	return result, nil
}

func makeBody(question string) string {
	parentMessageID, err := random.UUIdV4()
	if err != nil {
		parentMessageID = "aaa1165b-248b-4ed1-b99a-7e1ddfbd2b58"
	}
	var fullBody = `{
	"action": "next",
	"messages": [
		{
			"author": {
				"role": "user"
			},
			"content": {
				"content_type": "text",
				"parts": [
					` + strconv.Quote(question) + `
				]
			},
			"metadata": {}
		}
	],
	"parent_message_id": ` + strconv.Quote(parentMessageID) + `,
	"model": "text-davinci-002-render-sha",
	"timezone_offset_min": -480,
	"suggestions": [],
	"history_and_training_disabled": true,
	"arkose_token": null
}`

	return fullBody
}

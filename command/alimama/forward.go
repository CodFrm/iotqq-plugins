package alimama

import (
	"encoding/json"
	"errors"
	"github.com/CodFrm/iotqq-plugins/db"
	"github.com/CodFrm/iotqq-plugins/utils"
	"github.com/CodFrm/iotqq-plugins/utils/iotqq"
	"github.com/CodFrm/iotqq-plugins/utils/taobaoopen"
	"strconv"
	"strings"
	"time"
)

func ForwardGroup(args iotqq.Message) bool {
	group := args.CurrentPacket.Data.FromGroupID
	content := args.CurrentPacket.Data.Content
	if result, _ := db.Redis.Get("alimama:group:forward:enable").Result(); result != "1" {
		return false
	}
	if val, _ := db.Redis.HGet("alimama:forward:group", strconv.Itoa(group)).Result(); val != "1" {
		return false
	}
	//匹配淘口令发送
	tkl := utils.RegexMatch(content, ".(\\w{10,}).")
	if len(tkl) < 2 {
		return false
	}
	if tkl := utils.RegexMatch(content, "[$￥](\\w{10,})[$￥]"); len(tkl) > 0 {
		if strings.Index(content, "自助") != -1 {
			return false
		}
		Forward(args)
		return true
	}
	return false
}

func AddForwardGroup(group int) error {
	return db.Redis.HSet("alimama:forward:group", group, "1").Err()
}

func RemoveForwardGroup(group int) error {
	sgroup := strconv.Itoa(group)
	return db.Redis.HDel("alimama:forward:group", sgroup).Err()
}

func EnableGroupForward(enable bool) error {
	return db.Redis.Set("alimama:group:forward:enable", enable, 0).Err()
}

func Forward(args iotqq.Message) error {
	if args.CurrentPacket.Data.Content[:4] == "转 " {
		args.CurrentPacket.Data.Content = args.CurrentPacket.Data.Content[4:]
	}
	//非图片,直接转发
	list, err := db.Redis.SMembers("alimama:group:list").Result()
	if err != nil {
		return err
	}
	//单独的口令
	cmd := utils.RegexMatch(args.CurrentPacket.Data.Content, "^.(\\w{10,}).$")
	if len(cmd) > 0 {
		_, tkl, err := DealTkl(args.CurrentPacket.Data.Content)
		if err != nil {
			return err
		}
		url := tkl.Content[0].PictURL
		content := tkl.Content[0].TaoTitle + " " + tkl.Content[0].QuanhouJiage + "￥" + "\n" + tkl.Content[0].Tkl
		for _, v := range list {
			if url == "" {
				iotqq.QueueSendMsg(utils.StringToInt(v), 0, content)
			} else {
				iotqq.QueueSendPicMsg(utils.StringToInt(v), 0, content, url)
			}
		}
		mq.publisher(content)
		return nil
	}
	if args.CurrentPacket.Data.MsgType == "TextMsg" {
		var tkl *taobaoopen.ConverseTkl
		args.CurrentPacket.Data.Content, tkl, err = DealTkl(args.CurrentPacket.Data.Content)
		if err != nil && err.Error() != "很抱歉！商品ID解析错误！！！" {
			return err
		}
		if tkl != nil && IsTklSend(tkl) {
			return errors.New("重复发送")
		}
		for _, v := range list {
			iotqq.QueueSendMsg(utils.StringToInt(v), 0, args.CurrentPacket.Data.Content)
		}
		mq.publisher(args.CurrentPacket.Data.Content)
		return nil
	} else if args.CurrentPacket.Data.MsgType == "PicMsg" {
		pic := &iotqq.PicMsgContent{}
		if err := json.Unmarshal([]byte(args.CurrentPacket.Data.Content), pic); err != nil {
			return err
		}
		var err error
		var tkl *taobaoopen.ConverseTkl
		//处理口令
		pic.Content, tkl, err = DealTkl(pic.Content)
		if err != nil && err.Error() != "很抱歉！商品ID解析错误！！！" {
			return err
		}
		if tkl != nil && IsTklSend(tkl) {
			return errors.New("重复发送")
		}
		url := ""
		if pic.FriendPic == nil {
			url = pic.GroupPic[0].Url
		} else {
			url = pic.FriendPic[0].Url
		}
		for _, v := range list {
			iotqq.QueueSendPicMsg(utils.StringToInt(v), 0, pic.Content, url)
		}
		mq.publisher(pic.Content)
		return nil
	}
	return errors.New("不支持的类型")
}

func IsTklSend(tkl *taobaoopen.ConverseTkl) bool {
	return !db.Redis.SetNX("alimama:tkl:is:send:"+tkl.Content[0].TaoID, "1", time.Second*300).Val()
}
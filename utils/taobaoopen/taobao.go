package taobaoopen

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Taobao struct {
	AppKey    string
	AppSecret string
	Entrance  string
	AdzoneId  string
	ZtkAppKey string
	Sid       string
	Pid       string
}

func NewTaobao(config TaobaoConfig) *Taobao {
	return &Taobao{
		AppKey:    config.AppKey,
		AppSecret: config.AppSecret,
		Entrance:  config.Entrance,
		AdzoneId:  config.AdzoneId,
		ZtkAppKey: config.ZtkAppKey,
		Sid:       config.Sid,
		Pid:       config.Pid,
	}
}

func (t *Taobao) PublicFunc(method string, param ...*Kv) (string, error) {
	if param == nil {
		param = make([]*Kv, 0)
	}
	param = append(param, GenKv("method", method))
	param = append(param, GenKv("app_key", t.AppKey))
	param = append(param, GenKv("timestamp", time.Now().Format("2006-01-02 15:04:05")))
	param = append(param, GenKv("format", "json"))
	param = append(param, GenKv("v", "2.0"))
	param = append(param, GenKv("sign_method", "hmac-sha256"))

	sign := t.Sign(param)
	param = append(param, GenKv("sign", sign))
	return t.request(param)
}

func (t *Taobao) request(param []*Kv) (string, error) {
	data := ""
	for _, v := range param {
		data += v.Key + "=" + url.QueryEscape(v.Value) + "&"
	}
	data = data[:len(data)-1]
	resp, err := http.Post(t.Entrance, "application/x-www-form-urlencoded;charset=utf-8", bytes.NewBuffer([]byte(data)))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func (t *Taobao) Sign(param []*Kv) string {
	ret := ""
	sort.Sort(Kvs(param))
	for _, v := range param {
		ret += v.Key + v.Value
	}
	h := hmac.New(sha256.New, []byte(t.AppSecret))
	h.Write([]byte(ret))
	sha := hex.EncodeToString(h.Sum(nil))
	return strings.ToUpper(sha)
}

// https://open.taobao.com/api.htm?docId=35896&docType=2
func (t *Taobao) MaterialSearch(keyword string) ([]*MaterialItem, error) {
	str, err := t.PublicFunc("taobao.tbk.dg.material.optional", GenKv("adzone_id", t.AdzoneId), GenKv("q", keyword), GenKv("page_size", "5"))
	if err != nil {
		return nil, err
	}
	ret := MaterialSearchRespond{}
	if err := json.Unmarshal([]byte(str), &ret); err != nil {
		return nil, err
	}
	return ret.Respond.ResultList.MapData, nil
}

// https://open.taobao.com/api.htm?docId=31127&docType=2
func (t *Taobao) CreateTpwd(text string, url string) (string, error) {
	str, err := t.PublicFunc("taobao.tbk.tpwd.create", GenKv("text", text), GenKv("url", url))
	if err != nil {
		return "", err
	}
	ret := TpwdRespond{}
	if err := json.Unmarshal([]byte(str), &ret); err != nil {
		return "", err
	}
	return ret.Respond.Data.PasswordSimple, nil
}

type spreadRequests struct {
	Url string `json:"url"`
}

// https://open.taobao.com/api.htm?spm=a2e0r.13193907.0.0.43f224aduPPsqi&docId=27832&docType=2
func (t *Taobao) GetSpread(url []string) ([]*SpreadItem, error) {
	u := make([]*spreadRequests, 0)
	for _, v := range url {
		u = append(u, &spreadRequests{Url: v})
	}
	urlJson, _ := json.Marshal(u)
	str, err := t.PublicFunc("taobao.tbk.spread.get", GenKv("requests", string(urlJson)))
	if err != nil {
		return nil, err
	}
	ret := GetSpreadRespond{}
	if err := json.Unmarshal([]byte(str), &ret); err != nil {
		return nil, err
	}
	return ret.Respond.Results.TbkSpread, nil
}

// https://open.taobao.com/api.htm?docId=48340&docType=2&scopeId=18294
func (t *Taobao) GetActiveInfo(id string) (*GetActiveInfo, error) {
	str, err := t.PublicFunc("taobao.tbk.activity.info.get", GenKv("adzone_id", t.AdzoneId), GenKv("activity_material_id", id))
	if err != nil {
		return nil, err
	}
	ret := &GetActiveInfo{}
	if err := json.Unmarshal([]byte(str), &ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// http://www.zhetaoke.com/user/open/open_gaoyongzhuanlian_tkl.aspx
func (t *Taobao) ConversionTkl(tkl string) (*ConverseTkl, error) {
	resp, err := HttpGet("https://api.zhetaoke.com:10001/api/open_gaoyongzhuanlian_tkl.ashx?appkey="+
		t.ZtkAppKey+"&sid="+t.Sid+"&pid="+t.Pid+"&tkl="+url.QueryEscape(tkl)+"&signurl=5", nil)
	if err != nil {
		return nil, err
	}
	ret := &ConverseTkl{}
	if err := json.Unmarshal(resp, ret); err != nil {
		retErr := &ZtkError{}
		if err := json.Unmarshal(resp, retErr); err != nil {
			return nil, err
		}
		return nil, retErr
	}
	return ret, nil
}

// http://www.zhetaoke.com/user/open/open_shangpin_id.aspx
func (t *Taobao) ResolveTklAddress(tkl string) (*TklAddress, error) {
	resp, err := HttpGet("https://api.zhetaoke.com:10001/api/open_get_location.ashx?appkey="+
		t.ZtkAppKey+"&sid="+t.Sid+"&content="+url.QueryEscape(tkl)+"&type=1&jb=1&member_sid=", nil)
	if err != nil {
		return nil, err
	}
	ret := &TklAddress{}
	if err := json.Unmarshal(resp, ret); err != nil {
		retErr := &ZtkError{}
		if err := json.Unmarshal(resp, retErr); err != nil {
			return nil, err
		}
		return nil, retErr
	}
	return ret, nil
}

//http://www.zhetaoke.com/user/open/open_gaoyongzhuanlian.aspx
func (t *Taobao) ConversionShopId(id string) (*ConverseTkl, error) {
	resp, err := HttpGet("https://api.zhetaoke.com:10001/api/open_gaoyongzhuanlian.ashx?appkey="+
		t.ZtkAppKey+"&sid="+t.Sid+"&pid="+t.Pid+"&num_iid="+url.QueryEscape(id)+"&signurl=5", nil)
	if err != nil {
		return nil, err
	}
	ret := &ConverseTkl{}
	if err := json.Unmarshal(resp, ret); err != nil {
		retErr := &ZtkError{}
		if err := json.Unmarshal(resp, retErr); err != nil {
			return nil, err
		}
		return nil, retErr
	}
	return ret, nil
}

func (t *Taobao) QueryOrder(order ...OrderOption) ([]*OrderItem, *OrderQueryRespond, error) {
	o := &OrderOptions{}
	for _, v := range order {
		v(o)
	}
	url := "https://api.zhetaoke.com:10001/api/open_dingdanchaxun2.ashx?appkey=" + t.ZtkAppKey + "&sid=" +
		t.Sid + "&start_time=" + o.StartTime + "&end_time=" + o.EndTime + "&signurl=1"
	if o.PageNo > 0 {
		url += "&page_no=" + strconv.Itoa(o.PageNo)
	}
	resp, err := HttpGet(url, nil)
	if err != nil {
		return nil, nil, err
	}
	ret := &QueryOrderUrl{}
	if err := json.Unmarshal(resp, ret); err != nil {
		retErr := &ZtkError{}
		if err := json.Unmarshal(resp, retErr); err != nil {
			return nil, nil, err
		}
		return nil, nil, retErr
	}
	resp, err = HttpGet(ret.Url, nil)
	if err != nil {
		return nil, nil, err
	}
	respJson := &OrderQueryRespond{}
	if err := json.Unmarshal(resp, respJson); err != nil {
		retErr := &TaobaoError{}
		if err := json.Unmarshal(resp, retErr); err != nil {
			return nil, nil, err
		}
		return nil, nil, retErr
	}
	return respJson.TbkScOrderDetailsGetResponse.Data.Results.PublisherOrderDto, respJson, nil
}

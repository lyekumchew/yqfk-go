package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/robfig/cron/v3"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"
)

const (
	homeUrl = "https://cas.dgut.edu.cn/home/Oauth/getToken/appid/illnessProtectionHome/state/home.html"
	yqfkUrl = "https://yqfk.dgut.edu.cn/home/base_info/"
	scUrl   = "https://sc.ftqq.com/"
)

var (
	username = flag.String("u", "", "username")
	passwd   = flag.String("p", "", "password")
	sckey    = flag.String("k", "", "Server Chan Key")
)

func main() {
	flag.Parse()

	c := cron.New()
	_, err := c.AddFunc("CRON_TZ=Asia/Shanghai 10 6 * * *", func() { run() })
	if err != nil {
		serviceLogger(err.Error(), 1)
	}
	c.Start()

	run()

	_, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch := make(chan os.Signal, 2)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	select {
	case s := <-ch:
		cancel()
		fmt.Printf("\nreceived signal %s, exit.\n", s)
	}
}

func run() {
	token, err := getToken()
	if err != nil {
		serviceLogger(err.Error(), 1)
		scMsg("疫情防控: 失败", err.Error())
	} else {
		serviceLogger("获取 access_token 成功", 2)
		err = postForm(token)
		if err != nil {
			serviceLogger(err.Error(), 1)
			scMsg("疫情防控: 失败", err.Error())
		} else {
			serviceLogger("疫情上报成功", 0)
			scMsg("疫情防控: 成功", "")
		}
	}
}

func getToken() (token string, err error) {
	// http.client init
	jar, _ := cookiejar.New(nil)
	client := http.Client{Jar: jar}

	// xss token
	resp, err := client.Get(homeUrl)
	defer resp.Body.Close()
	if err != nil {
		return "", err
	}
	contents, _ := ioutil.ReadAll(resp.Body)
	re := regexp.MustCompile(`var token = "(.*?)";`)
	res := re.FindAllStringSubmatch(string(contents), -1)
	token = res[0][1]
	if token == "" {
		return "", errors.New("获取 access_token 失败")
	}

	// login information
	params := url.Values{}
	params.Set("username", *username)
	params.Set("password", *passwd)
	params.Set("__token__", token)

	// login
	req, _ := http.NewRequest("POST", homeUrl, strings.NewReader(params.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp2, err := client.Do(req)
	if err != nil {
		serviceLogger(err.Error(), 1)
		return "", err
	}
	defer resp2.Body.Close()
	contents2, _ := ioutil.ReadAll(resp2.Body)

	if strings.Contains(string(contents2), "通过") {
		serviceLogger("登陆成功", 2)
	} else {
		return "", errors.New("登陆失败")
	}

	// access token
	re = regexp.MustCompile(`"info":"(.*?)"}`)
	res = re.FindAllStringSubmatch(string(contents2), -1)
	token = res[0][1]
	token = strings.ReplaceAll(token, "\\", "")
	resp3, err := client.Get(token)
	defer resp3.Body.Close()
	if err != nil {
		serviceLogger(err.Error(), 1)
		return "", err
	}
	re = regexp.MustCompile(`access_token=(.*?)$`)
	res = re.FindAllStringSubmatch(resp3.Request.URL.String(), -1)
	token = res[0][1]
	if token == "" {
		return "", errors.New("获取 access_token 失败")
	}

	return token, nil
}

func postForm(token string) error {
	// http.client init
	jar, _ := cookiejar.New(nil)
	client := http.Client{Jar: jar}

	// get information
	req, _ := http.NewRequest("GET", yqfkUrl+"getBaseInfo", nil)
	req.Header.Set("authorization", "Bearer "+token)
	resp, err := client.Do(req)
	defer resp.Body.Close()
	if err != nil {
		return errors.New("发送获取个人信息请求失败")
	}
	contents, _ := ioutil.ReadAll(resp.Body)
	re := regexp.MustCompile(`"info":(.*)}`)
	res := re.FindAllStringSubmatch(string(contents), -1)
	info := res[0][1]
	if info == "" || info == "[]" {
		return errors.New("获取个人信息失败")
	}
	// 已经打卡
	if strings.Contains(info, "成功") || strings.Contains(info, "已提交") {
		serviceLogger("今日已提交，不进行任务处理", 2)
		return nil
	}
	req, _ = http.NewRequest("POST", yqfkUrl+"addBaseInfo", strings.NewReader(info))
	req.Header.Set("authorization", "Bearer "+token)
	resp, err = client.Do(req)
	if err != nil {
		return errors.New("提交失败")
	}
	contents, _ = ioutil.ReadAll(resp.Body)
	if strings.Contains(string(contents), "成功") || strings.Contains(string(contents), "已提交") {
		return nil
	} else {
		return errors.New("疫情防控提交失败")
	}
}

func scMsg(text, desp string) {
	client := http.Client{}
	resp, err := client.Get(scUrl + *sckey + ".send?text=" + url.QueryEscape(text) + "&desp=" + url.QueryEscape(desp))
	//resp, err := http.Get()
	defer resp.Body.Close()
	if err != nil && resp.Status != string(http.StatusOK) {
		serviceLogger("Server 酱发送失败", 1)
		serviceLogger(resp.Request.URL.String(), 2)
	}
	contents, _ := ioutil.ReadAll(resp.Body)
	if res, _ := regexp.MatchString("success", string(contents)); res != true {
		serviceLogger("Server 酱发送失败", 1)
		serviceLogger(string(contents), 2)
	}
}

func serviceLogger(m string, level int) {
	color := []string{"[SUCCESS]", "[ERROR]", "[INFO]"}
	header := []string{"\u001B[32;1m", "\u001B[31;1m", "\u001B[36;1m"}
	loc, _ := time.LoadLocation("Asia/Shanghai")
	fmt.Println(header[level], color[level], time.Now().In(loc).Format("2006-01-02 15:04:05"), m, "\033[0m")
}

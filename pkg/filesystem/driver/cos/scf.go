package cos

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"io"
	"io/ioutil"
	"net/url"
	"strconv"
	"strings"
	"time"

	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/hashid"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	scf "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/scf/v20180416"
)

const scfFunc = `# -*- coding: utf8 -*-
# SCF配置COS觸發，向 Cloudreve 發送回調
from qcloud_cos_v5 import CosConfig
from qcloud_cos_v5 import CosS3Client
from qcloud_cos_v5 import CosServiceError
from qcloud_cos_v5 import CosClientError
import sys
import logging
import requests

logging.basicConfig(level=logging.INFO, stream=sys.stdout)
logger = logging.getLogger()


def main_handler(event, context):
    logger.info("start main handler")
    for record in event['Records']:
        try:
            if "x-cos-meta-callback" not in record['cos']['cosObject']['meta']:
                logger.info("Cannot find callback URL, skiped.")
                return 'Success'
            callback = record['cos']['cosObject']['meta']['x-cos-meta-callback']
            key = record['cos']['cosObject']['key']
            logger.info("Callback URL is " + callback)

            r = requests.get(callback)
            print(r.text)

            

        except Exception as e:
            print(e)
            print('Error getting object {} callback url. '.format(key))
            raise e
            return "Fail"

    return "Success"
`

// CreateSCF 建立回調雲函數
func CreateSCF(policy *model.Policy, region string) error {
	// 初始化用戶端
	credential := common.NewCredential(
		policy.AccessKey,
		policy.SecretKey,
	)
	cpf := profile.NewClientProfile()
	client, err := scf.NewClient(credential, region, cpf)
	if err != nil {
		return err
	}

	// 建立回調程式碼資料
	buff := &bytes.Buffer{}
	bs64 := base64.NewEncoder(base64.StdEncoding, buff)
	zipWriter := zip.NewWriter(bs64)
	header := zip.FileHeader{
		Name:   "callback.py",
		Method: zip.Deflate,
	}
	writer, err := zipWriter.CreateHeader(&header)
	if err != nil {
		return err
	}
	_, err = io.Copy(writer, strings.NewReader(scfFunc))
	zipWriter.Close()

	// 建立雲函數
	req := scf.NewCreateFunctionRequest()
	funcName := "cloudreve_" + hashid.HashID(policy.ID, hashid.PolicyID) + strconv.FormatInt(time.Now().Unix(), 10)
	zipFileBytes, _ := ioutil.ReadAll(buff)
	zipFileStr := string(zipFileBytes)
	codeSource := "ZipFile"
	handler := "callback.main_handler"
	desc := "Cloudreve 用回調函數"
	timeout := int64(60)
	runtime := "Python3.6"
	req.FunctionName = &funcName
	req.Code = &scf.Code{
		ZipFile: &zipFileStr,
	}
	req.Handler = &handler
	req.Description = &desc
	req.Timeout = &timeout
	req.Runtime = &runtime
	req.CodeSource = &codeSource

	_, err = client.CreateFunction(req)
	if err != nil {
		return err
	}

	time.Sleep(time.Duration(5) * time.Second)

	// 建立觸發器
	server, _ := url.Parse(policy.Server)
	triggerType := "cos"
	triggerDesc := `{"event":"cos:ObjectCreated:Post","filter":{"Prefix":"","Suffix":""}}`
	enable := "OPEN"

	trigger := scf.NewCreateTriggerRequest()
	trigger.FunctionName = &funcName
	trigger.TriggerName = &server.Host
	trigger.Type = &triggerType
	trigger.TriggerDesc = &triggerDesc
	trigger.Enable = &enable

	_, err = client.CreateTrigger(trigger)
	if err != nil {
		return err
	}

	return nil
}

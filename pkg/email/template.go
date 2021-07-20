package email

import (
	"fmt"

	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/util"
)

// NewActivationEmail 建立啟動郵件
func NewActivationEmail(userName, activateURL string) (string, string) {
	options := model.GetSettingByNames("siteName", "siteURL", "siteTitle", "mail_activation_template")
	replace := map[string]string{
		"{siteTitle}":     options["siteName"],
		"{userName}":      userName,
		"{activationUrl}": activateURL,
		"{siteUrl}":       options["siteURL"],
		"{siteSecTitle}":  options["siteTitle"],
	}
	return fmt.Sprintf("【%s】註冊啟動", options["siteName"]),
		util.Replace(replace, options["mail_activation_template"])
}

// NewResetEmail 建立重設密碼郵件
func NewResetEmail(userName, resetURL string) (string, string) {
	options := model.GetSettingByNames("siteName", "siteURL", "siteTitle", "mail_reset_pwd_template")
	replace := map[string]string{
		"{siteTitle}":    options["siteName"],
		"{userName}":     userName,
		"{resetUrl}":     resetURL,
		"{siteUrl}":      options["siteURL"],
		"{siteSecTitle}": options["siteTitle"],
	}
	return fmt.Sprintf("【%s】密碼重設", options["siteName"]),
		util.Replace(replace, options["mail_reset_pwd_template"])
}

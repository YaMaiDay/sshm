package tui

import (
	"github.com/YaMaiDay/sshm/internal/config"
)

func (f addForm) fields() []formField {
	fields := []formField{
		{Label: "基础信息", Section: true},
		{ID: categoryFormIndex, Label: "分类", Value: f.Category},
		{ID: nameFormIndex, Label: "服务器名称", Value: f.Name},
		{Label: "目标服务器", Section: true},
		{ID: hostFormIndex, Label: "服务器地址", Value: f.HostName},
		{ID: userFormIndex, Label: "用户名", Value: f.User},
		{ID: portFormIndex, Label: "端口", Value: f.Port},
		{ID: identityFormIndex, Label: "服务器本地密钥文件", Value: f.IdentityFile},
		{ID: passwordFormIndex, Label: "密码", Value: f.Password},
	}
	if f.Category != config.BastionCategory {
		fields = append(fields,
			formField{Label: "跳板机", Section: true},
			formField{ID: jumpHostRefFormIndex, Label: "使用跳板机", Value: emptyChoice(f.JumpHostRef, "无")},
		)
	}
	fields = append(fields,
		formField{Label: "辅助信息", Section: true},
		formField{ID: noteFormIndex, Label: "备注", Value: f.Note},
		formField{ID: expireAtFormIndex, Label: "到期时间", Value: f.ExpireAt},
	)
	return fields
}

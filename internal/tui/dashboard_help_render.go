package tui

import "strings"

func (m Model) renderDashboardHelp(width int) string {
	if width < 1 {
		width = 1
	}
	parts := []string{
		"More ?",
		"Move ↑↓←→/hjkl",
		"Login Enter",
		"Details Space",
		"Command m",
		"Batch b",
		"History i",
		"Transfer y",
		"Deploy g",
		"Resources n",
		"Settings .",
		"Overview w",
		"View z",
		"Pin t",
		"Favorite f",
		"Favorites v",
		"Add a",
		"Copy c",
		"Edit e",
		"Delete x",
		"Upload u",
		"Download d",
		"Refresh r",
		"Search /",
		"Category Tab",
		"Online o",
		"Problems p",
		"Sort s",
		"Quit q",
	}
	if m.isChineseUI() {
		parts = []string{
			"更多 ?",
			"移动 ↑↓←→/hjkl",
			"登录 Enter",
			"详情 Space",
			"命令 m",
			"批量 b",
			"历史 i",
			"传输 y",
			"部署 g",
			"资源 n",
			"设置 .",
			"总览 w",
			"视图 z",
			"置顶 t",
			"收藏 f",
			"收藏 v",
			"添加 a",
			"复制 c",
			"编辑 e",
			"删除 x",
			"上传 u",
			"下载 d",
			"刷新 r",
			"搜索 /",
			"分类 Tab",
			"在线 o",
			"异常 p",
			"排序 s",
			"退出 q",
		}
	}
	help := strings.Join(parts, "  ")
	return helpStyle.Render(fit(help, width))
}

package ui

import (
	"os"
	"runtime"
	"strings"

	"github.com/egoist/mygo/ui"
)

// GeneralPage is the index of the app's own settings in pages.
const GeneralPage = 6

// general holds how go-voice itself behaves; the system's login items are the
// truth for opening at login, so the switch shows what they report.
func (v *View) general(c *ui.Context) {
	s := v.Snapshot
	group(c, "startup", "启动", nil, func() {
		row(c, "open-at-login", func() {
			ui.Box(c).Size(34, 34).Radius(17).Background(colPill).Center().Children(func() {
				ui.Icon(c, iconPower).Size(15, 15).TextColor(pick(s.OpenAtLogin, colGreen, colSubtle))
			})
			detail := "登录 Mac 后在菜单栏后台运行，不打开窗口；缺少必需权限时会打开“权限”页。"
			if runtime.GOOS == "linux" {
				detail = "登录桌面后在后台运行，不打开窗口；由系统的启动项负责，可在此关闭。"
			}
			ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
				ui.Text(c, "登录时打开").FontSize(14).FontWeight(500).SingleLine()
				ui.Text(c, detail).FontSize(12).TextColor(colMuted).LineHeight(1.4)
			})
			on := s.OpenAtLogin
			if ui.Switch(c, &on).Label("登录时打开 go-voice").Changed() {
				v.Actions.SetOpenAtLogin(on)
			}
		})
	})
	if runtime.GOOS != "linux" && s.LoginApproval {
		ui.Row(c.Key("login-approval")).Padding(12, 12, 12, 16).Gap(12).Radius(14).Background(colOrangeSoft).Children(func() {
			ui.Icon(c, iconWarn).Size(16, 16).TextColor(colOrange)
			ui.Text(c, "还需在“系统设置 › 通用 › 登录项”中允许 go-voice，允许后这里会自动更新。").FontSize(13).TextColor(ui.Hex("#FFC4A3")).Grow(1)
			if smallPill(c, "login-items", "打开登录项", pillPrimary, iconArrow, false) {
				v.Actions.OpenLoginItems()
			}
		})
	} else if s.LoginFeedback != "" {
		feedback(c, s.LoginFeedback, s.LoginFeedbackOK)
	}
	if runtime.GOOS == "linux" {
		notice(c, "linux-general", "Linux 预览版：用打包目录中的 install.sh 安装到 ~/.local/share/go-voice 并添加应用菜单；更新时先重新构建，再运行 install.sh。", false)
	} else {
		notice(c, "login-note", "也可以在“系统设置 › 通用 › 登录项”中关闭；这里的开关始终与系统设置一致。", false)
	}
	if s.Version != "" || s.RunPath != "" {
		group(c, "build", "运行版本", nil, func() {
			row(c, "run-path", func() {
				detail := s.RunPath
				if home, err := os.UserHomeDir(); err == nil && home != "" && strings.HasPrefix(detail, home) {
					detail = "~" + strings.TrimPrefix(detail, home)
				}
				label(c, "版本", pickText(s.Version == "", "开发构建", s.Version)+" · 运行于 "+detail)
			})
		})
	}
}

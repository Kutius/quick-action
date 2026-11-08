# 发版

1. 跑一遍测试，把 main 拉到最新：

   ```sh
   git pull --rebase origin main
   go test ./...
   ```

2. 改 `mygo.json` 里的 `version`，在 `CHANGELOG.md` 顶部加一节 `## <版本>`，用中文写用户能感觉到的变化。两者放进一个提交，提交信息就叫版本号。

   CHANGELOG 的这一节既是应用更新窗口里显示的说明，也是 release 页面的说明；没有它 `mygo build` 会直接失败。

3. 先推 main，再打 tag。一起推的话 main 被拒绝时 tag 照样会推上去，于是用不在 main 上的提交构建发布：

   ```sh
   git push origin main
   git tag v0.1.1
   git push origin v0.1.1
   ```

   main 被拒绝就再拉一次，把新进来的改动写进 changelog，amend 那个版本提交，测试，推 main，再打 tag。

4. tag 触发 `.github/workflows/release.yml`：跑测试，构建并签名 Windows 包，上传到一个**草稿** release。等它跑完：

   ```sh
   gh run watch "$(gh run list --workflow release.yml -L1 --json databaseId -q '.[0].databaseId')" --exit-status
   ```

5. 看一眼草稿里的文件：`quick-action-setup-<版本>-amd64.exe`、`quick-action-<版本>-windows-amd64.tar.gz`（更新装的就是它）、`update-windows-amd64.json`、`quick-action-<版本>-amd64.zip`。齐了就发布：

   ```sh
   gh release edit v0.1.1 --draft=false --latest
   ```

   草稿对已装的程序不可见，发布之后它们才会在下一次检查时看到。

6. 确认更新源里已经是新版本：

   ```sh
   curl -sL https://github.com/Kutius/quick-action/releases/latest/download/update-windows-amd64.json
   ```

## 更新签名密钥

更新包用 Ed25519 签名：公钥在 `mygo.json` 的 `updates.publicKey`，私钥只在 GitHub 的 `MYGO_UPDATER_PRIVATE_KEY` secret 里，CI 用它签名。本机那一份在 `%APPDATA%\mygo\update-keys\quick-action\mygo-update.key`，别丢，也别提交进仓库。

密钥不能随手换：已装的程序只认自己构建时烧进去的那把公钥，换了密钥签出来的包它们一律拒收。真要换，得先用旧密钥发一个带新公钥的版本，等用户升上去之后，再用新密钥签下一个。

## 注意

- 0.1.0 及更早的版本里没有更新器，不会自己升级；0.1.1 是第一个能自更新的版本。
- 安装包装在 `%LOCALAPPDATA%\Programs\quick-action`，用户自己有写权限，所以能原地更新。从压缩包里拿出来直接跑的，按 exe 所在的目录更新；那个目录不可写（比如 Program Files）时不会自动更新，检查更新会直接说明原因。

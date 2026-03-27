# Backend Run Guide

## 1) Yeu cau

- Windows (repo nay dang dong goi `nginx.exe` trong thu muc `nginx/`)
- Python 3
- Go (neu khong trong PATH thi set bien moi truong `GO_EXE`)
- MySQL co schema phu hop

## 2) Cau hinh DB

Ung dung doc cau hinh DB theo thu tu uu tien:

1. Bien moi truong (`DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASS`, `DB_NAME`, `DB_PARAMS`)
2. File `golang/main/database/config_1.json`
3. Gia tri mac dinh trong code

Vi du PowerShell (chi cho session hien tai):

```powershell
$env:DB_HOST="vip.tecom.pro"
$env:DB_PORT="3306"
$env:DB_USER="root"
$env:DB_PASS="123456"
$env:DB_NAME="hustmedi_777"
```

Neu muon may chay on dinh theo kieu global (luu bien moi truong lau dai), dung `setx`:

```powershell
setx DB_HOST "vip.tecom.pro"
setx DB_PORT "3306"
setx DB_USER "root"
setx DB_PASS "123456"
setx DB_NAME "hustmedi_777"
```

Luu y: sau khi `setx`, can mo cua so terminal moi de bien co hieu luc.

## 3) Chay nhanh (khuyen nghi)

Tu thu muc `backend/`:

```powershell
python server/main.py
```

Script nay se:

- stop instance cu (neu co)
- start Nginx (port `8794`)
- start Golang backend (port `8795`)
- bat watch file `.go`/`.json` de restart khi thay doi

## 4) Chay tung phan (neu can debug)

### Chi chay Golang backend

```powershell
cd golang
go run ./main
```

### Dev Go (khuyen nghi cho may dev) Chỉ dùng riêng GO

Tu thu muc `backend/`:

```powershell
python server/go_run.py
```

Che do nay se:

- stop process cu
- start lai Go backend
- bat watch file `.go`/`.json` de tu restart khi code thay doi

Neu may dev khong muon chay scheduler, set bien moi truong truoc khi start:

```powershell
$env:GOLANG_SCHEDULER="0"
python server/go_run.py
```

Neu can bat lai scheduler:

```powershell
$env:GOLANG_SCHEDULER="1"
python server/go_run.py
```

### Chi chay Nginx

```powershell
python server/nginx.py
```

## 5) Endpoint test nhanh

- Backend truc tiep: `http://127.0.0.1:8795/`
- Qua Nginx: `http://127.0.0.1:8794/go/api/test`
- Money update: `http://127.0.0.1:8794/go/service/money_update`
- Money list: `http://127.0.0.1:8794/go/service/money_list`

## 6) Log va kiem tra loi

- Golang: `server/logs/golang-launch.log`
- Nginx: `server/logs/nginx-launch.log`
- Neu port bi dung, stop process cu roi chay lai `python server/main.py`

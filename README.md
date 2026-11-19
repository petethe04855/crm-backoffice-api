# backoffice-api

API หลังบ้านของ CRM (สมาชิก LINE OA) และ **จุดเชื่อม ERP จุดเดียว** ของ CRM
แผน: [`plan/CRM_BACKOFFICE.md`](../plan/CRM_BACKOFFICE.md), [`plan/CRM_LINE_OA_MEMBERSHIP.md`](../plan/CRM_LINE_OA_MEMBERSHIP.md)

- Go + Fiber, กฎธุรกิจทั้งหมดอยู่ใน [`../crm-core`](../crm-core) (`replace crm-core => ../crm-core` ใน `go.mod`)
- ฐานข้อมูล PostgreSQL DB `crm`, schema `crm` — ตารางทุกตัวได้ prefix `crm.` จาก naming strategy
- พนักงาน login ด้วยบัญชี ERP: ตรวจ JWT ของ ERP + เช็คสถานะบัญชีกับ ERP `/auth/me` (cache 60 วิ) + role ของ CRM จาก `crm.staff_roles`

## รัน

```bash
cp .env.example .env   # แล้วคัดลอก DB_PASSWORD และ JWT_SECRET จาก erp-api-v2/.env
go run ./cmd/api
```

ชื่อตัวแปรเหมือน `erp-api-v2` (`DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `JWT_SECRET`) แต่ **ไม่ใช้ `DB_NAME` / `DATABASE_URL` ของ ERP** — CRM ใช้ฐานข้อมูลของตัวเอง `CRM_DB_NAME` (ค่าเริ่มต้น `crm`) ซึ่งระบบสร้างให้เองตอนเปิดครั้งแรกถ้ายังไม่มี (user ต้องมีสิทธิ์ CREATEDB) แล้ว migration สร้าง schema `crm` และตารางต่อ
ตัวแปรขึ้นต้น `CRM_` (เช่น `CRM_DB_PASSWORD`, `ERP_JWT_SECRET`) ยังใช้ได้และมีลำดับความสำคัญสูงกว่า

Production ควรแยก role ของ CRM เอง — ดู [`scripts/setup-db.sql`](scripts/setup-db.sql)

### Worker และ upload

- `WORKER_ENABLED` (ค่าเริ่มต้น `true`) เปิด/ปิดตัวรันงานเบื้องหลังใน process เดียวกับ API — ปิดได้เมื่อรันหลาย instance หรือต้องการ stop ชั่วคราว
- `UPLOAD_DIR` (ค่าเริ่มต้น `./uploads`) ที่เก็บไฟล์รูป สร้าง directory ให้เองตอนเริ่ม; โฟลเดอร์นี้อยู่ใน `.gitignore`

งานเบื้องหลังใช้ `internal/worker`: แต่ละ job มี goroutine ของตัวเอง จึง **ไม่มีการรันซ้อน** — รอบที่ช้ากว่าจะข้าม tick ที่ผ่านมาระหว่างนั้น (ตั้งใจ: งานที่วางแผนไว้เช่น coupon/point expiry ไม่ต้องการ backlog) กำหนดได้ทั้ง `Every` (ทุก N) และ `At` (เวลารายวันตาม `Asia/Bangkok`) โมดูลใหม่เพิ่ม job ใน `workerJobs` ที่ `cmd/api/main.go` (หนึ่งบรรทัด)

## ทดสอบ

```bash
go test ./...
```

Route test (`internal/delivery/http/route`) รันทั้ง API กับ crm-core จริงบน SQLite in-memory และจำลอง ERP `/auth/me`

## Endpoints

| Method | Path | สิทธิ์ |
|---|---|---|
| GET | `/api/v1/health` | public |
| GET | `/api/v1/crm/me` | staff ทุกคน |
| GET | `/api/v1/crm/tiers` | `members.view` |
| GET | `/api/v1/crm/members` (`?search&status&tierId&registeredFrom&registeredTo&page&limit`, `?format=xlsx`) | `members.view` (+`members.export` สำหรับ xlsx) |
| GET | `/api/v1/crm/members/:id` | `members.view` |
| GET | `/api/v1/crm/promotions` (`?month=YYYY-MM&status&placement&search&page&limit`) | `promotions.view` |
| GET | `/api/v1/crm/promotions/:id` | `promotions.view` |
| POST / PATCH / DELETE | `/api/v1/crm/promotions[/:id]` | `promotions.manage` (ลบเฉพาะ draft) |
| POST | `/api/v1/crm/promotions/:id/publish` \| `/archive` \| `/duplicate` | `promotions.manage` |
| POST | `/api/v1/crm/uploads/images` (multipart field `file`, jpg/png/webp ≤ 2 MB) | `promotions.manage` หรือ `rewards.manage` |
| GET | `/api/v1/files/:id` | public (uuid เป็น key ที่เดาไม่ได้, ใช้ใน `<img>`) |
| GET / PUT / DELETE | `/api/v1/crm/staff-roles[/:erpUserId]` | `staff_roles.manage` (owner) |

ตารางสิทธิ์: [`internal/authz/permission.go`](internal/authz/permission.go)

## โครงสร้าง

```text
cmd/api/                      main: config, DB, migrations, routes, listen/shutdown
cmd/api/wire.go               ประกอบ handler ต่อโมดูล (wireMember, wireUpload, …)
config/                       env config + Validate()
internal/authz/               permission matrix
internal/client/erp/          HTTP client ไป erp-api-v2 (ตอนนี้มี /auth/me)
internal/session/             cache ผลเช็ค session กับ ERP
internal/storage/             ที่เก็บไฟล์ upload (local disk ก่อน, เปลี่ยนเป็น S3 ได้)
internal/worker/              ตัวรัน job เบื้องหลัง (interval / รายวันเวลาไทย)
internal/delivery/http/       handler, dto, middleware, route (ไฟล์ละโมดูล), export (xlsx)
pkg/jwt, pkg/response         ตรวจ JWT ของ ERP, standard response
```

Route แยกไฟล์ละโมดูล (`route/member.go`, `staff_role.go`, `sales_dashboard.go`, `upload.go`, `promotion.go`) — โมดูลใหม่เพิ่ม field ใน `route.Handlers`, ฟังก์ชัน `registerXxx` ในไฟล์ของตัวเอง และหนึ่งบรรทัดใน `route.Register`; migration เพิ่มไฟล์ `models_<module>.go` แล้ว append หนึ่งบรรทัดใน `migrations.Models`

## ยังไม่ได้ทำ (ตามแผน)

- job จริงของ worker (แต้มหมดอายุ, สถานะโปรโมชั่น, คูปองหมดอายุ) — โมดูล T2/T3/T4 จะเพิ่มเอง
- `/internal/*` ให้ ERP เรียก และ `/integrations/crm/*` ฝั่ง ERP — P3
- บันทึก audit การส่งออกลงตาราง (ตอนนี้ log `[AUDIT]` ลง stdout)
- ERP ยังเซ็น JWT แบบ HS256 จึงต้องแชร์ `ERP_JWT_SECRET`; ควรเปลี่ยนเป็น RS256

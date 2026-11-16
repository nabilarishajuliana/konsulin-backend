# KonsulIn Backend

Sistem reservasi slot bimbingan dosen. Go + Fiber, Clean Architecture tanpa controller.

## Kontrak bersama (wajib diikuti semua anggota)

- Module: konsulin-backend
- Response sukses: helper.Success(c, status, message, data)
- Response gagal: helper.Fail(c, status, code, message); validasi: helper.ErrorResponse
- Error code: lihat helper/errcode.go (SLOT_FULL, ALREADY_BOOKED, VERSION_CONFLICT, dll)
- Arsitektur: model, repository (interface + impl menerima helper.DBTX), service (pegang *fiber.Ctx). Tanpa controller.
- Transaksi: repository menerima helper.DBTX; TxManager.WithTx membungkus satu transaksi
- Outbox event: booking.created, booking.cancelled, booking.swapped
- Routing: semua endpoint lewat route.Register; sediakan fungsi registrasi sub-route, jangan edit route.go bersamaan
- Git: branch feat/boilerplate (Risha), feat/slot-booking (Adel), feat/auth-worker (Juan); semua lewat PR ke main

## Menjalankan

cp .env.example .env    # isi DB_PASSWORD dan JWT_SECRET
docker compose up -d --build
curl http://localhost:8080/api/v1/health

## Pembagian

- Risha: config, database, helper, middleware, route, migrations (users, refresh_tokens, outbox), outbox relay, docker
- Adel: slot dan booking (model, repository, service, migrations slots dan bookings)
- Juan: auth, RBAC, notes (MongoDB), worker Asynq
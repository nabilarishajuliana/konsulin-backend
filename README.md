# KonsulIn — Sistem Reservasi Slot Bimbingan Dosen

Backend KonsulIn adalah sistem reservasi dan manajemen slot bimbingan akademik yang dirancang untuk mencegah overbooking saat beberapa mahasiswa memesan slot bimbingan dosen secara bersamaan. Sistem ini mengadopsi pola Clean Architecture tanpa lapisan controller, dengan kontrol konkurensi tingkat baris PostgreSQL dan pola Transactional Outbox untuk pemrosesan event yang andal.

Proyek ini adalah tugas akhir mata kuliah Praktikum Pemrograman Backend Lanjut — D4 Teknik Informatika, Universitas Airlangga.

---

## Tim

| Nama | NIM | Peran | Domain |
|---|---|---|---|
| Nabila Risha Juliana | 434241017 | Git Master & Core Architect | Boilerplate, infrastruktur, outbox relay, Docker |
| Adella Putri Asy'ari | 434241023 | Transaction Engine Developer | Slot dan Booking (concurrency, atomic swap) |
| Juanne Meilanie Raya Maturbongs | 434241026 | Auth, Data & Worker Developer | Authentication, MongoDB, Asynq worker |

---

## Tech Stack

| Kategori | Teknologi |
|---|---|
| Bahasa & framework | Go, Fiber v2 |
| Arsitektur | Clean Architecture (tanpa controller) |
| Database transaksional | PostgreSQL 16 (pgxpool) |
| Database dokumen | MongoDB 7 |
| Queue & background job | Redis + Asynq |
| Keamanan | JWT (HS256), bcrypt, RBAC |
| Dokumentasi API | Swagger (OpenAPI) |
| Deployment | Docker, Docker Compose |
| Testing | Go testing |

---

## Fitur Utama

**Mahasiswa**
- Melihat daftar slot bimbingan yang tersedia
- Memesan slot dengan proteksi anti-overbooking (SELECT FOR UPDATE)
- Konfirmasi kehadiran sebelum sesi berlangsung
- Membatalkan atau menukar (swap) booking secara atomik
- Menulis feedback setelah sesi

**Dosen**
- Membuka, mengubah kapasitas, dan menutup slot konsultasi
- Melihat daftar mahasiswa pemesan slot
- Menandai status kehadiran (attended / no-show)
- Menulis catatan hasil bimbingan

**Admin Prodi**
- Mengelola akun pengguna dan peran
- Memantau rekap penggunaan slot
- Memantau antrean Asynq dan outbox

**Sistem (latar belakang)**
- Transactional Outbox: setiap perubahan booking dicatat atomik ke tabel outbox dalam satu transaksi, lalu diteruskan ke Asynq oleh relay terpisah
- Pengingat H-1 otomatis sebelum sesi
- Auto-release slot bila mahasiswa tidak mengkonfirmasi 60 menit sebelum sesi
- Optimistic locking (version) saat dosen mengubah kapasitas slot

---

## Arsitektur

```
┌──────────┐      ┌───────────────────────────────────┐
│  Client  │ ───▶ │ Fiber Router + Middleware         │
└──────────┘      │ (RequireAuth, RequirePermission)  │
                  └──────────────┬────────────────────┘
                                 │
                        ┌────────▼────────┐
                        │ Service Layer    │
                        │ (business rules) │
                        └────────┬────────┘
                                 │
                   ┌─────────────┼─────────────┐
                   ▼             ▼             ▼
            ┌──────────┐  ┌──────────┐  ┌──────────┐
            │PostgreSQL│  │ MongoDB  │  │  Outbox  │
            │ (ACID)   │  │(notes)   │  │  table   │
            └──────────┘  └──────────┘  └─────┬────┘
                                              │ (polling)
                                        ┌─────▼─────┐
                                        │Outbox     │
                                        │Relay      │
                                        └─────┬─────┘
                                              │ (enqueue)
                                        ┌─────▼─────┐
                                        │Redis +    │
                                        │Asynq      │
                                        └─────┬─────┘
                                              │ (consume)
                                        ┌─────▼─────┐
                                        │Asynq      │
                                        │Worker     │
                                        └───────────┘
```

Service layer memegang `*fiber.Ctx` langsung (tanpa controller), repository menerima `helper.DBTX` sehingga query yang sama dapat berjalan dalam transaksi atau di luar transaksi.

---

## Struktur Folder

```
konsulin-backend/
├── app/
│   ├── model/          # entitas dan DTO
│   ├── repository/     # interface + implementasi PostgreSQL & MongoDB
│   └── service/        # business logic, memegang *fiber.Ctx
├── config/             # env loader, logger
├── database/           # koneksi PostgreSQL, MongoDB, Redis, runner migrasi
├── helper/             # JWTManager, PermissionSet, response builder, DBTX, TxManager
├── middleware/         # RequireAuth, RequirePermission, RequireJSON, rate limiter
├── migrations/         # file SQL berurutan (dijalankan otomatis saat startup)
├── outbox/             # Outbox Relay (polling + enqueue Asynq)
├── route/              # route.Register + struct Dependencies
├── docs/               # hasil swag init (Swagger OpenAPI)
├── Dockerfile
├── docker-compose.yml
├── .env.example
├── main.go             # entrypoint role=api
└── worker.go           # entrypoint role=worker
```

---

## Setup dan Menjalankan

### Prasyarat

- Docker Desktop (sudah mencakup Docker Engine dan Docker Compose)
- Git

Tidak perlu memasang Go, PostgreSQL, MongoDB, atau Redis di laptop — semua dijalankan di dalam container.

### Langkah

```bash
# 1. Clone repository
git clone https://github.com/nabilarisha/konsulin-backend.git
cd konsulin-backend

# 2. Salin file environment dan isi secret
cp .env.example .env
# Edit .env, isi JWT_SECRET (minimal 32 karakter acak) dan DB_PASSWORD
# JWT_SECRET dapat dihasilkan dengan: openssl rand -hex 32

# 3. Jalankan seluruh sistem
docker compose up -d --build

# 4. Verifikasi
curl http://localhost:8080/api/v1/health
```

Respons yang diharapkan:
```json
{"success":true,"message":"server berjalan","data":null}
```

Setelah service naik, Swagger UI tersedia di `http://localhost:8080/swagger/index.html` (bila `SWAGGER_ENABLED=true` di `.env`).

---

## Daftar Endpoint

Base path: `/api/v1`. Seluruh response berbentuk `{ success, message, data }` untuk sukses dan `{ success, code, message }` untuk gagal.

### Autentikasi

| Method | Path | Role | Fungsi |
|---|---|---|---|
| POST | `/auth/register` | Publik | Registrasi akun baru |
| POST | `/auth/login` | Publik | Login, menghasilkan access + refresh token |
| POST | `/auth/refresh` | Publik | Tukar refresh token (rotation) |
| POST | `/auth/logout` | Login | Cabut refresh token |
| GET | `/auth/me` | Login | Profil dan daftar permission pengguna |

### Slot

| Method | Path | Role | Fungsi |
|---|---|---|---|
| GET | `/slots` | Semua login | Daftar slot aktif |
| POST | `/slots` | Dosen, Admin | Buka slot baru |
| GET | `/slots/:id` | Semua login | Detail slot |
| PATCH | `/slots/:id/capacity` | Dosen, Admin | Ubah kapasitas (optimistic lock) |
| DELETE | `/slots/:id` | Dosen, Admin | Tutup slot |
| GET | `/slots/:id/bookings` | Dosen pemilik, Admin | Daftar mahasiswa pemesan |

### Booking

| Method | Path | Role | Fungsi |
|---|---|---|---|
| POST | `/slots/:id/book` | Mahasiswa | Pesan slot (FOR UPDATE) |
| DELETE | `/slots/:id/book` | Mahasiswa, Admin | Batal booking |
| PATCH | `/bookings/:id/confirm` | Mahasiswa | Konfirmasi hadir sebelum sesi |
| POST | `/bookings/swap` | Mahasiswa | Pindah slot secara atomik |
| GET | `/bookings/mine` | Mahasiswa | Riwayat booking milik sendiri |
| PATCH | `/bookings/:id/status` | Dosen, Admin | Tandai attended / no_show |

### Notes (MongoDB)

| Method | Path | Role | Fungsi |
|---|---|---|---|
| POST | `/bookings/:id/notes` | Pihak terkait | Tulis catatan atau feedback konsultasi |
| GET | `/bookings/:id/notes` | Pihak terkait, Admin | Baca catatan konsultasi |

### Admin

| Method | Path | Role | Fungsi |
|---|---|---|---|
| GET | `/users` | Admin | Daftar pengguna |
| POST | `/users` | Admin | Tambah pengguna |
| GET | `/users/:id` | Admin | Detail pengguna |
| PATCH | `/users/:id` | Admin | Perbarui pengguna |
| PATCH | `/users/:id/role` | Admin | Ubah peran pengguna |
| DELETE | `/users/:id` | Admin | Hapus pengguna |
| GET | `/admin/queue` | Admin | Monitor antrean Asynq dan outbox |

Daftar error code (SLOT_FULL, ALREADY_BOOKED, VERSION_CONFLICT, QUOTA_BELOW_BOOKED, BOOKING_LIMIT_EXCEEDED, SLOT_CLOSED, dst.) didefinisikan di `helper/errcode.go` dan dipakai konsisten di seluruh service.

---

## Perintah Operasional

```bash
# Status seluruh container
docker compose ps

# Mengikuti log API dan worker
docker compose logs -f api worker

# Build ulang dan ganti container api + worker (setelah ubah kode)
docker compose up -d --build api worker

# Lihat tabel PostgreSQL
docker compose exec postgres psql -U postgres -d konsulin -c "\dt"

# Masuk psql interaktif
docker compose exec -it postgres psql -U postgres -d konsulin

# Masuk MongoDB shell
docker compose exec mongo mongosh konsulin

# Berhenti (data tetap ada)
docker compose stop

# Hapus container (data tetap ada)
docker compose down

# Reset total termasuk data
docker compose down -v
```

Migrasi baru cukup ditaruh di folder `migrations/` dengan penamaan berurutan (`00X_nama.sql`); runner migrasi di aplikasi akan menjalankannya otomatis saat container `api` naik.

---

## Konvensi Kontribusi

- Nama module Go: `konsulin-backend`. Semua import internal memakai path ini.
- Branch utama `main` dilindungi; perubahan masuk melalui Pull Request.
- Branch kerja per anggota: `feat/boilerplate` (Risha), `feat/slot-booking` (Adel), `feat/auth-worker` (Juan).
- Format pesan commit mengikuti Conventional Commits (`feat`, `fix`, `chore`, `test`, `ci`, ...).
- Service memegang `*fiber.Ctx`; repository menerima `helper.DBTX` sehingga dapat berjalan di dalam atau di luar transaksi.
- Operasi atomik (booking, swap, pencatatan outbox) memakai `helper.TxManager.WithTx`.

---

## Lisensi

Proyek akademik untuk tujuan pembelajaran. Hak cipta pada masing-masing anggota kelompok.
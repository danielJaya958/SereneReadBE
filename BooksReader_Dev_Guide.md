# **Books Reader \- Multiplatform Development Guide**

**Version:** 1.0.0

**Status:** Backend Ready (Production Grade)

Dokumen ini berfungsi sebagai panduan teknis untuk pengembangan sisi klien (Client-Side) pada platform **Android (Kotlin)**, **Desktop (DevExpress .NET)**, dan **Web (React JS)**. Dokumen ini didasarkan pada Backend Golang Enterprise yang telah selesai dibangun.

## **1\. Backend Architecture Overview**

Sebelum masuk ke implementasi klien, pahami bahwa Backend bekerja dengan spesifikasi berikut:

* **Base URL:** http://localhost:8080/api/v1 (Sesuaikan dengan IP Server saat deploy)  
* **Authentication:** JWT (JSON Web Token). Token berlaku 24 jam.  
* **File Handling:** Static File Server untuk PDF dan Thumbnail.  
* **Performance:** Menggunakan Redis Caching & Gzip Compression. Klien **wajib** mendukung header Accept-Encoding: gzip.

### **Struktur Backend (Referensi)**

Struktur ini menjamin keamanan dan performa data yang akan dikonsumsi klien.

/internal  
  ├── handlers/   (Endpoint controller)  
  ├── service/    (Validasi logic & Business rules)  
  ├── worker/     (Background process: Thumbnail Generator)

## **2\. API Authentication Strategy**

Semua platform klien harus menerapkan **Auth Interceptor** (Middleware di sisi klien) untuk menyisipkan token pada setiap request ke endpoint yang dilindungi.

* **Header Key:** Authorization  
* **Header Value Format:** Bearer \<token\_jwt\_disini\>

### **A. Login Flow**

1. User input Email & Password.  
2. POST ke /auth/login.  
3. Terima respons JSON berisi token.  
4. **Penyimpanan Token (Storage Strategy):**  
   * **Web (React):** localStorage atau HttpOnly Cookie (lebih aman).  
   * **Android (Kotlin):** EncryptedSharedPreferences.  
   * **Desktop (.NET):** Properties.Settings (User Scope) atau Windows Credential Manager.

### **B. Handling 401 Unauthorized**

Jika API mengembalikan status **401**:

1. Hapus Token dari storage.  
2. Redirect user paksa ke halaman Login.  
3. Tampilkan pesan "Sesi habis, silakan login kembali".

## **3\. Implementasi Fitur Utama & Logika Klien**

### **A. Books Management (List & Cache)**

Backend menggunakan **Redis Caching**.

* **Client Logic:**  
  * Saat melakukan Create, Update, atau Delete buku, Backend akan otomatis menghapus cache lama (Invalidate).  
  * Klien cukup melakukan GET /books ulang (Refresh) setelah operasi write sukses untuk mendapatkan data terbaru.

### **B. Upload PDF (Large File Handling)**

Backend sanggup menerima file hingga **75 MB**.

* **Content-Type:** multipart/form-data  
* **Field Name:** file (Wajib sesuai dengan backend).  
* **Timeout:** Set timeout HTTP Client (Axios/Retrofit/HttpClient) minimal **60-120 detik** untuk upload file besar, guna menghindari *Request Timeout* sebelum upload selesai.

### **C. PDF Rasterization (Thumbnail)**

Anda tidak perlu membuat thumbnail di sisi klien.

* **Backend Process:** Menggunakan **Asynq worker** \+ **Poppler**.  
* **Client Logic:** Respons API /books akan menyertakan URL gambar cover (cover\_url). Klien cukup me-load URL tersebut menggunakan library gambar:  
  * **React:** \<img src={book.cover\_url} /\>  
  * **Android:** Library **Coil** atau **Glide**.  
  * **Desktop:** PePictureEdit (DevExpress).

### **D. Smart Bookmark (Debounce Logic) \- CRITICAL**

Fitur ini menyimpan halaman terakhir (last\_read\_page). Untuk mencegah spam request ke server setiap kali user ganti halaman, terapkan **Debounce**.

**Algoritma Debounce (Wajib di semua Client):**

1. User pindah halaman PDF (misal: hal 5 \-\> 6 \-\> 7).  
2. **JANGAN** langsung kirim API request.  
3. Simpan nomor halaman sementara di *State/Variable*.  
4. Tunggu selama **3 \- 5 detik** (Timer).  
5. Jika user tidak pindah halaman lagi dalam durasi tersebut, baru kirim PATCH /books/{id}/progress.  
6. Jika user pindah lagi sebelum waktu habis, reset timer.

## **4\. Panduan Spesifik Platform**

### **📱 Android (Kotlin)**

**Tech Stack:** Retrofit, Coroutines, Room (Opsional), Android PdfViewer.

1. **Networking:**  
   * Gunakan **Retrofit** dengan OkHttpClient.  
   * Pasang AuthInterceptor pada OkHttp untuk menyisipkan header Bearer Token.  
2. **Concurrency:**  
   * Gunakan **Kotlin Coroutines** (viewModelScope.launch) untuk memanggil API agar tidak memblokir Main Thread (UI).  
3. **PDF Viewing:**  
   * Gunakan AndroidPdfViewer atau render manual menggunakan PdfRenderer API bawaan Android.  
   * Integrasikan *listener* onPageChange dengan fungsi *Debounce* (gunakan Handler atau Flow operators) untuk update progress.

### **💻 Desktop (DevExpress .NET / C\#)**

**Tech Stack:** Visual Studio 2022, DevExpress WinForms/WPF, RestSharp / HttpClient.

1. **UI Component:**  
   * Gunakan **DevExpress PDF Viewer Control**. Ini komponen powerful yang sudah built-in fitur reading.  
   * Gunakan **DevExpress GridControl** (Card View) untuk menampilkan daftar buku dengan Cover yang menarik.  
2. **API Handling:**  
   * Buat ApiService singleton menggunakan HttpClient.  
   * Gunakan Task.Run atau async/await untuk operasi I/O agar aplikasi tidak *Not Responding*.  
3. **Bookmark Logic:**  
   * Subscribe ke event PdfViewer.CurrentPageNumberChanged.  
   * Gunakan System.Timers.Timer untuk implementasi debounce sebelum menembak API update progress.

### **🌐 Web (React JS)**

**Tech Stack:** Vite, Axios, TanStack Query (React Query), React-PDF.

1. **State Management:**  
   * Gunakan **TanStack Query** untuk fetching data buku. Ini sangat cocok dengan backend Anda yang menggunakan Cache Redis, karena TanStack Query juga memiliki caching di sisi browser (stale-while-revalidate).  
2. **PDF Rendering:**  
   * Gunakan react-pdf.  
   * **PENTING:** Atur workerSrc pdf.js dengan benar agar performa render cepat.  
3. **Effect Hooks:**  
   * Gunakan useEffect untuk mendeteksi perubahan halaman.  
   * Gunakan setTimeout di dalam useEffect untuk debounce, dan jangan lupa clearTimeout pada *cleanup function*.

## **5\. API Endpoint Reference (Summary)**

Berikut adalah daftar endpoint utama yang telah tersedia di Backend:

### **Auth**

|

| **Method** | **Endpoint** | **Deskripsi** | **Body** |

| POST | /auth/register | Mendaftar user baru | {email, password} |

| POST | /auth/login | Login & Dapatkan Token | {email, password} |

### **Books**

| **Method** | **Endpoint** | **Deskripsi** | **Tipe Body** |

| GET | /books | Ambil semua buku (User scope) | \- |

| POST | /books | Upload buku baru | multipart/form-data |

| GET | /books/{id} | Detail buku & URL PDF | \- |

| PUT | /books/{id} | Update metadata (Judul/Tahun) | JSON |

| DELETE | /books/{id} | Hapus buku & file | \- |

### **Progress (Smart Bookmark)**

| **Method** | **Endpoint** | **Deskripsi** | **Body** |

| PATCH | /books/{id}/progress | Update halaman terakhir | {last\_read\_page: int} |

## **6\. Deployment Checklist (Future)**

Saat backend siap di-deploy ke server (VPS/Cloud), pastikan config klien diubah:

1. **Base URL:** Ubah dari localhost ke domain publik (misal: https://api.booksreader.com/v1).  
2. **SSL/HTTPS:** Klien Android modern (API 28+) menolak koneksi HTTP biasa (Cleartext) secara default. Pastikan Backend di-deploy dengan SSL (HTTPS).  
3. **CORS:** Backend sudah mendukung CORS (github.com/go-chi/cors), pastikan domain frontend Web dimasukkan ke whitelist backend jika diperlukan pengaturan ketat.

*Selamat Mengembangkan Frontend\! Backend Anda sudah "Enterprise Ready", sekarang saatnya membuat UI yang sepadan.*
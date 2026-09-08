# Plan: Integrasi Smart Subtitles untuk dao-clip

Tujuan: Meningkatkan akurasi dan estetika subtitle agar sinkron dengan audio TTS dan nyaman dibaca di format vertikal.

## Tahap 1: Alignment (Sinkronisasi Audio-Subtitle)
- [ ] Tambahkan langkah pasca-TTS: Menjalankan `whisper.cpp` pada file audio output TTS (`.wav` hasil TTS) untuk mendapatkan *timestamp* yang sangat akurat.
- [ ] Parsing hasil JSON dari Whisper untuk membuat file `.ass` baru yang berbasis waktu nyata (bukan estimasi).

## Tahap 2: Logika Line-Breaking (Estetika)
- [ ] Kembangkan fungsi `SplitSubtitleText(text string) string` di Go yang memecah kalimat berdasarkan:
  - Tanda baca (koma, titik, tanda seru).
  - Panjang karakter maksimal (misal: 15-20 karakter per baris).
  - Prioritas jeda alami kalimat.

## Tahap 3: Update Renderer FFmpeg
- [ ] Modifikasi `services/ffmpeg_renderer.go` agar membaca file `.ass` yang telah dioptimalkan (bukan lagi *auto-generated* dari teks murni).
- [ ] Pastikan *margin* tetap konsisten dengan profil `dao-clip`.

## Tahap 4: Verifikasi
- [ ] Uji coba pada klip 30 detik.
- [ ] Perbandingan kualitas: *Old vs New Alignment*.
- [ ] Validasi *subtitle* tidak bertumpuk atau terpotong di layar vertikal.

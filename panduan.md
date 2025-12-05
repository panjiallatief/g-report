Panduan Perhitungan Metrik IT Ops (SLA, MTTA, MTTR)Dokumen ini menjelaskan definisi, rumus matematika, dan implementasi SQL untuk metrik performa utama dalam sistem IT Broadcast Ops.1. MTTA (Mean Time To Acknowledge)Definisi: Rata-rata waktu yang dibutuhkan dari saat tiket dibuat hingga respon pertama dari teknisi (manusia). Ini mengukur "Kesigapan Tim".Pemicu Awal: created_at (Saat tiket masuk).Pemicu Akhir: first_response_at (Saat teknisi membalas chat pertama kali atau mengubah status menjadi IN_PROGRESS).Pengecualian: Balasan otomatis (Auto-reply bot) TIDAK dihitung.Rumus$$\text{MTTA} = \frac{\sum (\text{first\_response\_at} - \text{created\_at})}{\text{Jumlah Tiket yang Direspon}}$$Implementasi SQLSELECT 
    AVG(EXTRACT(EPOCH FROM (first_response_at - created_at))/60) AS avg_mtta_minutes
FROM tickets
WHERE 
    first_response_at IS NOT NULL 
    AND created_at >= NOW() - INTERVAL '30 days'; -- Filter Periode
2. MTTR (Mean Time To Resolve)Definisi: Rata-rata waktu yang dibutuhkan untuk menyelesaikan masalah sepenuhnya (dari tiket dibuat sampai tiket dinyatakan selesai). Ini mengukur "Kompetensi Teknis".Pemicu Awal: created_at.Pemicu Akhir: resolved_at (Saat status berubah menjadi RESOLVED atau CLOSED).Catatan Handover: Jika tiket dioper antar shift (Handover), waktu tetap berjalan terus. MTTR dihitung berdasarkan total durasi tiket hidup, bukan durasi per orang.Rumus$$ \text{MTTR} = \frac{\sum (\text{resolved_at} - \text{created_at})}{\text{Jumlah Tiket yang Selesai}} $$Implementasi SQLSELECT 
    AVG(EXTRACT(EPOCH FROM (resolved_at - created_at))/60) AS avg_mttr_minutes
FROM tickets
WHERE 
    status IN ('RESOLVED', 'CLOSED')
    AND resolved_at IS NOT NULL
    AND created_at >= NOW() - INTERVAL '30 days';
3. SLA (Service Level Agreement)Definisi: Janji tingkat layanan berdasarkan urgensi. Dalam konteks Broadcast, kita menggunakan Matrix Prioritas.Matrix Target (SLA Goals)Prioritas / KonteksTarget Respon (MTTA)Target Penyelesaian (MTTR)Urgent (ON AIR)< 2 Menit< 15 MenitHigh (Pra-Siaran)< 10 Menit< 1 JamNormal (Office)< 60 Menit< 8 JamSLA Compliance Rate (Tingkat Kepatuhan)Persentase tiket yang diselesaikan di dalam batas waktu yang ditentukan.Rumus$$ \text{SLA Rate} = \left( \frac{\text{Jumlah Tiket Patuh}}{\text{Total Tiket}} \right) \times 100% $$Implementasi Logic (Go/Backend)Untuk menentukan apakah sebuah tiket "Patuh" (Compliant) atau "Melanggar" (Breached):Tentukan Deadline:Jika urgency = 'ON_AIR_EMERGENCY', maka deadline = created_at + 15 minutes.Jika urgency = 'NORMAL', maka deadline = created_at + 8 hours.Cek Status:Jika resolved_at <= deadline, maka COMPLIANT.Jika resolved_at > deadline ATAU (NOW() > deadline dan belum selesai), maka BREACHED.Implementasi SQL (SLA Report)SELECT 
    priority,
    COUNT(*) as total_tickets,
    -- Hitung berapa yang patuh (Contoh untuk Urgent 15 menit)
    COUNT(*) FILTER (
        WHERE (resolved_at - created_at) <= INTERVAL '15 minutes'
    ) as compliant_tickets,
    -- Persentase
    ROUND(
        (COUNT(*) FILTER (WHERE (resolved_at - created_at) <= INTERVAL '15 minutes')::decimal / COUNT(*)) * 100, 
    2) as sla_percentage
FROM tickets
WHERE priority = 'URGENT_ON_AIR'
GROUP BY priority;
4. Studi Kasus (Contoh Nyata)Skenario:09:00: Tiara melaporkan "Mic Studio 1 Mati" (Status: Urgent ON AIR).09:02: Budi (IT) membalas: "Siap, otw lokasi." -> MTTA tercatat: 2 menit.09:02 - 09:10: Budi memperbaiki kabel.09:10: Masalah selesai, Budi klik "Resolve". -> MTTR tercatat: 10 menit.Analisis:MTTA: 2 Menit (Target < 2 Menit) -> SLA Respon Tercapai (Pas).MTTR: 10 Menit (Target < 15 Menit) -> SLA Resolusi Tercapai (Aman).Kesimpulan: Tiket ini berkontribusi positif (Hijau) ke dashboard Manager.
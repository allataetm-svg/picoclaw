# Kod Yapısı

**Analiz Tarihi:** 2026-02-26

## Dizin Yapısı

```
picoclaw/
├── cmd/picoclaw/          # CLI komut implementasyonları
├── pkg/                   # Çekirdek paketler
│   ├── agent/             # AI ajan mantığı
│   ├── auth/              # Kimlik doğrulama
│   ├── bus/               # Mesaj veri yolu
│   ├── channels/          # Mesajlaşma platformu entegrasyonları
│   ├── config/            # Yapılandırma yönetimi
│   ├── constants/         # Sabitler
│   ├── cron/              # Zamanlanmış görevler
│   ├── devices/           # Donanım cihazı desteği
│   ├── gateway/           # HTTP sunucusu
│   ├── health/            # Sağlık kontrolleri
│   ├── heartbeat/         # Heartbeat servisi
│   ├── logger/            # Günlük kaydı
│   ├── migrate/           # Migrasyon araçları
│   ├── pairing/           # Cihaz eşleştirme
│   ├── providers/         # LLM sağlayıcı implementasyonları
│   ├── routing/          # Mesaj yönlendirme
│   ├── session/           # Oturum yönetimi
│   ├── skills/            # Beceri sistemi
│   ├── state/             # Kalıcı durum
│   ├── tools/             # Araç implementasyonları
│   ├── utils/             # Yardımcı fonksiyonlar
│   └── voice/             # Ses işleme
├── config/                # Yapılandırma örnekleri
├── workspace/              # Çalışma alanı dizini (oluşturulur)
├── go.mod                 # Go modül tanımı
└── Makefile               # Build komutları
```

## Dizin Amaçları

**`cmd/picoclaw/`:**
- Amaç: CLI komut giriş noktaları
- İçerik: `main.go`, `cmd_*.go` dosyaları
- Önemli dosyalar:
  - `main.go` - Komut yönlendirmeli ana giriş noktası
  - `cmd_gateway.go` - Gateway/sunucu komutu
  - `cmd_onboard.go` - Başlatma komutu
  - `cmd_agent.go` - Etkileşimli ajan modu
  - `cmd_auth.go` - Kimlik doğrulama yönetimi
  - `cmd_status.go` - Durum görüntüleme
  - `cmd_cron.go` - Cron görev yönetimi
  - `cmd_pairing.go` - Cihaz eşleştirme
  - `cmd_skills.go` - Beceri yönetimi

**`pkg/agent/`:**
- Amaç: Çekirdek AI ajan implementasyonu
- İçerik: Ajan döngüsü, örnek, kayıt, bağlam, bellek
- Önemli dosyalar:
  - `loop.go` - Ana mesaj işleme döngüsü (1301 satır)
  - `instance.go` - Ajan örneği temsili
  - `registry.go` - Ajan kaydı ve yönlendirme
  - `context.go` - LLM için bağlam oluşturma
  - `memory.go` - Bellek/bağlam yönetimi

**`pkg/channels/`:**
- Amaç: Çoklu platform mesajlaşma entegrasyonları
- İçerik: 12+ platform için kanal implementasyonları
- Önemli dosyalar:
  - `manager.go` - Kanal yöneticisi
  - `base.go` - Temel kanal arayüzü
  - `telegram.go`, `discord.go`, `slack.go` - Platform entegrasyonları

**`pkg/providers/`:**
- Amaç: LLM sağlayıcı implementasyonları
- İçerik: Çoklu sağlayıcı tipleri
- Önemli dosyalar:
  - `factory.go` - Sağlayıcı fabrikası
  - `types.go` - Ortak tipler
  - `claude_provider.go` - Anthropic Claude
  - `github_copilot_provider.go` - GitHub Copilot
  - `fallback.go` - Sağlayıcı fallback zinciri

**`pkg/tools/`:**
- Amaç: Ajanların kullanabileceği araçlar
- İçerik: Araç implementasyonları
- Önemli dosyalar:
  - `base.go` - Araç arayüzü
  - `registry.go` - Araç kaydı
  - `web.go` - Web arama/çekme
  - `shell.go` - Shell çalıştırma
  - `filesystem.go` - Dosya işlemleri
  - `spawn.go` - Alt ajan oluşturma
  - `skills_*.go` - Beceri araçları

**`pkg/skills/`:**
- Amaç: Genişletilebilir beceri sistemi
- İçerik: Beceri yükleme, kurulum, kayıt
- Önemli dosyalar:
  - `loader.go` - Beceri yükleyici
  - `installer.go` - Beceri kurucu
  - `registry.go` - Beceri kaydı

**`pkg/config/`:**
- Amaç: Yapılandırma yönetimi
- İçerik: Yapılandırma yapısı ve yükleme
- Önemli dosyalar:
  - `config.go` - Ana yapılandırma (722 satır)
  - `defaults.go` - Varsayılan yapılandırma
  - `migration.go` - Yapılandırma migrasyonu

**`pkg/bus/`:**
- Amaç: Dahili mesaj iletişimi
- İçerik: Mesaj veri yolu implementasyonu

**`pkg/gateway/`:**
- Amaç: HTTP sunucusu
- İçerik: HTTP sunucu implementasyonu

## Önemli Dosya Konumları

**Giriş Noktaları:**
- `cmd/picoclaw/main.go` - CLI giriş noktası
- `cmd/picoclaw/cmd_gateway.go` - Gateway komutu (birincil sunucu)

**Yapılandırma:**
- `pkg/config/config.go` - Yapılandırma yapısı tanımı
- `config/config.example.json` - Örnek yapılandırma

**Çekirdek Mantık:**
- `pkg/agent/loop.go` - Ajan mesaj işleme
- `pkg/providers/factory.go` - Sağlayıcı oluşturma
- `pkg/channels/manager.go` - Kanal yönetimi

**Testler:**
- Test dosyaları kaynak ile aynı konumda (örn. `loop_test.go`, `config_test.go`)

## İsimlendirme Kuralları

**Dosyalar:**
- Alt çizgili küçük harf: `agent_loop.go`, `config_test.go`
- Komutlar: `cmd_*.go`
- Kanal/platform spesifik: `{platform}.go` (telegram.go, discord.go)

**Dizinler:**
- Küçük harf, tek kelime veya ortak isimler: `agent`, `channels`, `tools`
- Çoğul koleksiyonlar için: `channels`, `providers`

**Paketler:**
- Açıklayıcı tek kelimeler: `agent`, `channels`, `providers`, `bus`
- Dizin adlarıyla tutarlı

## Yeni Kod Nereye Eklenir

**Yeni Kanal Entegrasyonu:**
- Implementasyon: `pkg/channels/{platform}.go`
- Testler: `pkg/channels/{platform}_test.go`
- Kayıt: `pkg/channels/manager.go`'da

**Yeni LLM Sağlayıcı:**
- Implementasyon: `pkg/providers/{sağlayıcı}_provider.go`
- Testler: `pkg/providers/{sağlayıcı}_test.go`
- Kayıt: `pkg/providers/factory.go`'da

**Yeni Araç:**
- Implementasyon: `pkg/tools/{araç_adı}.go`
- Testler: `pkg/tools/{araç_adı}_test.go`
- Takip: `pkg/tools/base.go`'daki araç arayüzü

**Yeni CLI Komutu:**
- Implementasyon: `cmd/picoclaw/cmd_{komut}.go`
- Kayıt: `cmd/picoclaw/main.go` switch ifadesinde

**Yeni Beceri:**
- `pkg/skills/` üzerinden yönetilir - beceriler çalışma alanından yüklenir

## Özel Dizinler

**`workspace/`:**
- Amaç: Ajan dosyaları için varsayılan çalışma alanı
- Oluşturulur: Evet, onboard sırasında
- Commit: Hayır, .gitignore'da

**`config/`:**
- Amaç: Örnek yapılandırmalar
- Oluşturulur: Hayır
- Commit: Evet

---

*Yapı analizi: 2026-02-26*

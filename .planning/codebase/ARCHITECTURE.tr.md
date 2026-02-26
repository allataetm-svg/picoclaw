# Mimari

**Analiz Tarihi:** 2026-02-26

## Proje Genel Bakış

PicoClaw, birden fazla mesajlaşma kanalını çeşitli LLM sağlayıcılarına bağlayan ultra hafif bir kişisel AI ajan çerçevesidir. Go dilinde yazılmıştır ve nanobot'tan (HKUDS) ilham alır.

## Desen Genel Bakış

**Genel:** Olay yönelimli, mesaj veri yolu mimarisi ile eklenti tabanlı kanal entegrasyonları

**Temel Özellikler:**
- Mesaj yönlendirme ile dahili olay veri yolu (`pkg/bus`)
- Kanaldan bağımsız tasarım - 12+ mesajlaşma platformunu destekler
- Çoklu sağlayıcı LLM desteği ile fallback zincirleri
- Genişletilebilir beceri sistemi ile araç destekli ajanlar
- Oturum tabanlı konuşma bağlamı yönetimi

## Katmanlar

**CLI/Komut Katmanı (`cmd/picoclaw/`):**
- Amaç: Komut satırı arayüzü giriş noktaları
- Konum: `cmd/picoclaw/`
- İçerik: Ana CLI komutları (onboard, agent, gateway, status, auth, cron, pairing, skills)
- Bağımlılıklar: Tüm çekirdek paketler
- Kullanım: Son kullanıcılar CLI üzerinden

**Gateway Katmanı (`pkg/gateway/`):**
- Amaç: Webhook almak ve WebUI sunmak için HTTP sunucusu
- Konum: `pkg/gateway/`
- İçerik: `server.go` - HTTP sunucu implementasyonu
- Bağımlılıklar: `pkg/channels`, `pkg/bus`
- Kullanım: Kanal webhook'ları ve isteğe bağlı WebUI

**Kanal Katmanı (`pkg/channels/`):**
- Amaç: Mesajlaşma platformu entegrasyonları
- Konum: `pkg/channels/`
- İçerik: Bireysel kanal implementasyonları (telegram.go, discord.go, slack.go, vs.)
- Bağımlılıklar: `pkg/bus`, `pkg/config`
- Kullanım: Gateway mesajları almak için

**Ajan Katmanı (`pkg/agent/`):**
- Amaç: Çekirdek AI ajan mantığı ve mesaj işleme
- Konum: `pkg/agent/`
- İçerik: `loop.go` - ana ajan döngüsü, `instance.go` - ajan örneği, `registry.go` - ajan kaydı, `context.go` - bağlam oluşturma
- Bağımlılıklar: `pkg/providers`, `pkg/tools`, `pkg/skills`, `pkg/bus`, `pkg/session`
- Kullanım: Gateway komutu

**Sağlayıcı Katmanı (`pkg/providers/`):**
- Amaç: LLM sağlayıcı soyutlamaları ve implementasyonları
- Konum: `pkg/providers/`
- İçerik: Çoklu sağlayıcı implementasyonları (claude_provider.go, openai_provider.go, anthropic, vs.)
- Bağımlılıklar: Harici LLM API'leri
- Kullanım: Ajan katmanı

**Araçlar Katmanı (`pkg/tools/`):**
- Amaç: Ajanların eylemleri gerçekleştirmek için kullanabileceği araçlar
- Konum: `pkg/tools/`
- İçerik: Araç implementasyonları (web.go, shell.go, filesystem.go, spawn.go, skills_*.go, vs.)
- Bağımlılıklar: Çeşitli sistem kaynakları
- Kullanım: Ajan katmanı

**Beceriler Katmanı (`pkg/skills/`):**
- Amaç: Genişletilebilir ajan yetenekleri için beceri yönetim sistemi
- Konum: `pkg/skills/`
- İçerik: `loader.go` - beceri yükleme, `installer.go` - beceri kurulum, `registry.go` - beceri kaydı
- Bağımlılıklar: Dosya sistemi, ağ
- Kullanım: Araç katmanı, CLI komutları

**Destek Katmanları:**
- `pkg/config/` - Yapılandırma yükleme ve yönetimi
- `pkg/bus/` - İletişim için dahili mesaj veri yolu
- `pkg/session/` - Konuşma oturumu yönetimi
- `pkg/state/` - Kalıcı durum yönetimi
- `pkg/routing/` - Mesaj yönlendirme mantığı
- `pkg/cron/` - Zamanlanmış görev yönetimi
- `pkg/devices/` - Donanım cihazı desteği (USB izleme)

## Veri Akışı

**Gelen Mesaj Akışı:**
1. Kanal mesajı alır (webhook/websocket/polling)
2. Kanal mesajı `InboundMessage` olarak veri yoluna yayınlar
3. Ajan döngüsü veri yolundan tüketir
4. Yönlendirme hangi ajanın mesajı işleyeceğini belirler
5. Ajan döngüsü LLM + araçlarla işler
6. Yanıt `OutboundMessage` olarak veri yoluna yayınlanır
7. Kanal yanıtı platforma gönderir

**Yapılandırma Akışı:**
1. CLI komutu çağrılır
2. Yapılandırma JSON dosyasından yüklenir (`~/.picoclaw/config.json`)
3. Ortam değişkenleri JSON'u geçersiz kılar (caarlos0/env üzerinden)
4. Yapılandırma doğrulanır ve bileşenlere geçirilir

## Temel Soyutlamalar

**Mesaj Veri Yolu:**
- Amaç: Dahili olaylarla bileşenleri ayırmak
- Örnekler: `pkg/bus/bus.go`
- Desen: Tür yapısı mesajları ile pub/sub

**LLM Sağlayıcı Arayüzü:**
- Amaç: Farklı LLM API'leri için tek tip arayüz
- Örnekler: `pkg/providers/types.go`, `pkg/providers/factory.go`
- Desen: Sağlayıcı implementasyonları ile Factory deseni

**Araç Kaydı:**
- Amaç: Ajanların kullanabileceği araçları kaydetme ve çalıştırma
- Örnekler: `pkg/tools/registry.go`
- Desen: Eklenti mimarisi

**Kanal Yöneticisi:**
- Amaç: Çoklu kanal örneklerini yönetme
- Örnekler: `pkg/channels/manager.go`
- Desen: Kanal arayüzü ile Yönetici deseni

## Giriş Noktaları

**Ana Gateway (Birincil):**
- Konum: `cmd/picoclaw/main.go` → `cmd_gateway.go`'da `gatewayCmd()`
- Tetikleme: `picoclaw gateway` komutu
- Sorumluluklar: HTTP sunucusunu başlatma, tüm bileşenleri başlatma, mesajları işleme

**Onboard:**
- Konum: `cmd/picoclaw/cmd_onboard.go`
- Tetikleme: `picoclaw onboard` komutu
- Sorumluluklar: Yapılandırma ve çalışma alanını başlatma

**Ajan CLI:**
- Konum: `cmd/picoclaw/cmd_agent.go`
- Tetikleme: `picoclaw agent` komutu
- Sorumluluklar: Etkileşimli ajan modu

## Hata Yönetimi

**Strateji:** Bağlamsal günlük kaydı ile hata sarmalama

**Desenler:**
- Fonksiyonlardan bağlamsal hatalar döndürülür
- Uygun seviyelerde günlük kaydı (Info, Warn, Error)
- Graceful degradation (örn. fallback sağlayıcılar)
- Yanıtlarda kullanıcı dostu hata mesajları

## Çapraz Kesen Endişeler

**Günlük Kaydı:** Yapılandırılmış günlük kaydı ile `pkg/logger`'da özel logger (InfoCF, WarnCF, ErrorCF, DebugCF)

**Doğrulama:** `pkg/config/config.go`'da yapılandırma doğrulama

**Kimlik Doğrulama:** `pkg/auth/`'da OAuth ve API anahtarı yönetimi

---

*Mimari analiz: 2026-02-26*

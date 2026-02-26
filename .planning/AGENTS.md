# AGENTS.md

PicoClaw projesi için geliştirme ajanı kuralları ve iş akışı rehberi.

## Genel Prensipler

### Özellik/Değişiklik İsteği Geldiğinde

1. **Planla** — İlk önce değişikliği planla
2. **Gerçekleştir** — Planı adım adım uygula
3. **Build Et** — Projeyi linux arm64 ve linux amd64 için derle
4. **Güncelle** — Proje dokümanlarını güncelle
5. **Commit** — Değişiklikleri commitle
6. **Push** — Main branch'a pushla

## Planlama Aşaması

- Değişikliğin kapsamını belirle
- Etkilenecek dosyaları tespit et
- Bağımlılıkları kontrol et
- Olası sorunları önceden değerlendir

## Uygulama Aşaması

- Planı adım adım takip et
- Her adımı test et
- Kod kalitesini koru
- Lint/typecheck çalıştır

## Build Aşaması

Değişiklikler tamamlandıktan sonra projeyi iki platform için derle:

```bash
# Linux AMD64
GOOS=linux GOARCH=amd64 go build -o picoclaw-linux-amd64 ./cmd/picoclaw

# Linux ARM64
GOOS=linux GOARCH=arm64 go build -o picoclaw-linux-arm64 ./cmd/picoclaw
```

Build başarılı olmalı. Hata varsa düzelt ve tekrar dene.

## Dokümantasyon

- API değişikliklerini güncelle
- README varsa güncelle
- Yeni dosyalar için yorum ekle
- CHANGELOG varsa güncelle

## Git İş Akışı

```bash
# Build et (doküman güncellemeden önce)
GOOS=linux GOARCH=amd64 go build -o picoclaw-linux-amd64 ./cmd/picoclaw
GOOS=linux GOARCH=arm64 go build -o picoclaw-linux-arm64 ./cmd/picoclaw

# Değişiklikleri commit et
git add .
git commit -m "feat: yeni özellik açıklaması"

# Main branch'a push et
git push origin main
```

## Kontrol Listesi

- [ ] Plan oluşturuldu
- [ ] Kod değişiklikleri yapıldı
- [ ] Testler geçti
- [ ] Lint/typecheck temiz
- [ ] Linux AMD64 build edildi
- [ ] Linux ARM64 build edildi
- [ ] Dokümanlar güncellendi
- [ ] Commit yapıldı
- [ ] Push edildi

---

*Son güncellenme: 2026-02-26 (build adımı eklendi)*

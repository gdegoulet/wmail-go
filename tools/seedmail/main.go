// seedmail dépose des messages de démonstration dans une boîte de test (GreenMail) par SMTP.
// Usage : go run ./tools/seedmail -addr greenmail:3025 -to test@localhost -n 40
package main

import (
	"bytes"
	"encoding/base64"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"log"
	"mime"
	"strings"
	"time"

	"github.com/emersion/go-smtp"
)

// enc encode un en-tête non ASCII (RFC 2047) : « réunion » devient =?UTF-8?q?r=C3=A9union?=.
func enc(s string) string { return mime.QEncoding.Encode("UTF-8", s) }

func msg(from, to, subject string, date time.Time, headers, body string) string {
	if i := strings.Index(from, " <"); i > 0 {
		from = enc(from[:i]) + from[i:]
	}
	subject = enc(subject)
	return fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nDate: %s\r\nMessage-ID: <%d.%s>\r\nMIME-Version: 1.0\r\n%s\r\n%s\r\n",
		from, to, subject, date.Format(time.RFC1123Z), date.UnixNano(), "seed@wmail-go", headers, body)
}

// gradient produit une image de démonstration (dégradé), en PNG ou JPEG, encodée en base64 sur des lignes de 76 caractères.
func gradient(w, h int, hue uint8, asJPEG bool) string {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 255 / w), G: uint8(y * 255 / h), B: hue, A: 255})
		}
	}
	var buf bytes.Buffer
	if asJPEG {
		_ = jpeg.Encode(&buf, img, nil)
	} else {
		_ = png.Encode(&buf, img)
	}
	enc := base64.StdEncoding.EncodeToString(buf.Bytes())
	var sb strings.Builder
	for len(enc) > 76 {
		sb.WriteString(enc[:76] + "\r\n")
		enc = enc[76:]
	}
	return sb.String() + enc
}

func main() {
	addr := flag.String("addr", "greenmail:3025", "serveur SMTP")
	to := flag.String("to", "test@localhost", "destinataire")
	n := flag.Int("n", 40, "nombre de messages de remplissage")
	flag.Parse()

	now := time.Now()
	var all []string
	all = append(all,
		msg("Bruno Martin <bruno@exemple.fr>", *to, "Compte rendu de la réunion", now.Add(-2*time.Hour),
			"Content-Type: text/plain; charset=UTF-8\r\n",
			"Bonjour,\r\n\r\nVoici le compte rendu : voir https://exemple.fr/cr-2026.\r\n\r\n> Peux-tu relire avant demain ?\r\n> Merci d'avance.\r\n\r\nCordialement,\r\nBruno"),
		msg("Boutique <promo@boutique.example>", *to, "Offre spéciale : -30 % aujourd'hui", now.Add(-5*time.Hour),
			"Content-Type: text/html; charset=UTF-8\r\n",
			`<html><body style="font-family:Arial;color:#333"><h2 style="color:#c00">-30 % sur tout le site</h2><p>Profitez-en <a href="https://boutique.example/promo">maintenant</a>.</p>`+
				`<img src="https://tracker.example/pixel.gif?u=42" width="1" height="1"><img src="https://boutique.example/banniere.jpg" alt="Bannière"><script>alert('xss')</script></body></html>`),
		msg("Claire Durand <claire@exemple.fr>", *to, "Facture de septembre", now.Add(-26*time.Hour),
			"Content-Type: multipart/mixed; boundary=\"BND\"\r\n",
			"--BND\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\nVous trouverez la facture en pièce jointe.\r\n--BND\r\nContent-Type: application/pdf; name=\"facture-septembre.pdf\"\r\n"+
				"Content-Disposition: attachment; filename=\"facture-septembre.pdf\"\r\nContent-Transfer-Encoding: base64\r\n\r\nJVBERi0xLjQKJSBmYXV4IHBkZgo=\r\n--BND--"),
		msg("Pascaline Jaouen <pascaline@exemple.fr>", *to, "Photos de vacances", now.Add(-3*time.Hour),
			"Content-Type: multipart/mixed; boundary=\"PH\"\r\n",
			"--PH\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\nVoici quelques photos et le programme.\r\n"+
				"--PH\r\nContent-Type: image/png; name=\"Screenshot_2025-08-11_plage.png\"\r\nContent-Disposition: attachment; filename=\"Screenshot_2025-08-11_plage.png\"\r\nContent-Transfer-Encoding: base64\r\n\r\n"+gradient(320, 200, 60, false)+"\r\n"+
				"--PH\r\nContent-Type: image/jpeg; name=\"montagne.jpg\"\r\nContent-Disposition: attachment; filename=\"montagne.jpg\"\r\nContent-Transfer-Encoding: base64\r\n\r\n"+gradient(400, 300, 200, true)+"\r\n"+
				"--PH\r\nContent-Type: application/pdf; name=\"programme.pdf\"\r\nContent-Disposition: attachment; filename=\"programme.pdf\"\r\nContent-Transfer-Encoding: base64\r\n\r\nJVBERi0xLjQKJSBmYXV4IHBkZgo=\r\n--PH--"),
		msg("Éloïse <eloise@exemple.fr>", *to, "Gala <script>alert(1)</script> & co", now.Add(-30*time.Hour),
			"Content-Type: text/plain; charset=UTF-8\r\n", "Sujet piégé : l'interface doit tout échapper."),
	)
	senders := []string{"David Petit <david@exemple.fr>", "Emma Roux <emma@exemple.fr>", "Service client <noreply@service.example>", "Farid Benali <farid@exemple.fr>"}
	for i := 1; i <= *n; i++ {
		all = append(all, msg(senders[i%len(senders)], *to, fmt.Sprintf("Message de démonstration %02d", i), now.Add(-time.Duration(40+i*7)*time.Hour),
			"Content-Type: text/plain; charset=UTF-8\r\n", strings.Repeat("Ceci est un message de test. ", 3)))
	}

	for i, m := range all {
		c, err := smtp.Dial(*addr)
		if err != nil {
			log.Fatal(err)
		}
		if err := c.SendMail("seed@wmail-go", []string{*to}, bytes.NewReader([]byte(m))); err != nil {
			log.Fatalf("message %d : %v", i+1, err)
		}
		_ = c.Quit()
	}
	fmt.Printf("%d messages déposés dans %s via %s\n", len(all), *to, *addr)
}

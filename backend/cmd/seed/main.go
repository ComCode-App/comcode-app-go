// Command seed provides a modular database seeder for default and demo data.
//
// Usage (repo root):
//
//	make seed
//	make seed STAGE=social-platforms
//	make seed STAGE=subscriber-user
//	make seed STAGE=profiles
//
// Or from backend/:
//
//	go run ./cmd/seed --stage=all
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/config"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/password"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Seeder func(ctx context.Context, pool *pgxpool.Pool) error

type Stage struct {
	Name string
	Run  Seeder
}

var stages = []Stage{
	{
		Name: "social-platforms",
		Run:  seedSocialPlatforms,
	},
	{
		Name: "subscriber-user",
		Run:  seedSubscriberUser,
	},
	{
		Name: "profiles",
		Run:  seedSubscriberProfiles,
	},
}

func main() {
	stageFlag := flag.String("stage", "all", "seeder stage to run: all, social-platforms, subscriber-user, profiles")
	flag.Parse()

	targetStage := strings.ToLower(strings.TrimSpace(*stageFlag))

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool, err := database.NewPostgresPool(ctx, cfg.DB)
	if err != nil {
		log.Fatalf("database connection: %v", err)
	}
	defer pool.Close()

	executed := 0
	for _, s := range stages {
		if targetStage == "all" || targetStage == s.Name {
			fmt.Printf("--> Running seeder [%s]...\n", s.Name)
			if err := s.Run(ctx, pool); err != nil {
				log.Fatalf("seeder [%s] failed: %v", s.Name, err)
			}
			fmt.Printf("✔ Seeder [%s] completed.\n", s.Name)
			executed++
		}
	}

	if executed == 0 {
		fmt.Printf("Unknown stage: %s\nAvailable stages: all", targetStage)
		for _, s := range stages {
			fmt.Printf(", %s", s.Name)
		}
		fmt.Println()
		os.Exit(1)
	}

	fmt.Printf("\nSeeding finished successfully (%d stage(s) executed).\n", executed)
}

type socialPlatformSeed struct {
	Name         string
	Slug         string
	Category     string
	URLPattern   string
	IconURL      string
	Placeholder  string
	Description  string
	DisplayOrder int
	IsActive     bool
}

func seedSocialPlatforms(ctx context.Context, pool *pgxpool.Pool) error {
	platforms := []socialPlatformSeed{
		{
			Name:         "LinkedIn",
			Slug:         "linkedin",
			Category:     "professional",
			URLPattern:   "https://linkedin.com/in/{value}",
			IconURL:      "https://www.google.com/s2/favicons?domain=linkedin.com&sz=128",
			Placeholder:  "username or profile-slug",
			Description:  "Professional networking and career profile",
			DisplayOrder: 1,
			IsActive:     true,
		},
		{
			Name:         "GitHub",
			Slug:         "github",
			Category:     "developer",
			URLPattern:   "https://github.com/{value}",
			IconURL:      "https://www.google.com/s2/favicons?domain=github.com&sz=128",
			Placeholder:  "username",
			Description:  "Open source repositories and developer portfolio",
			DisplayOrder: 2,
			IsActive:     true,
		},
		{
			Name:         "X (Twitter)",
			Slug:         "twitter",
			Category:     "social",
			URLPattern:   "https://x.com/{value}",
			IconURL:      "https://www.google.com/s2/favicons?domain=x.com&sz=128",
			Placeholder:  "handle (without @)",
			Description:  "Short-form social updates and microblogging",
			DisplayOrder: 3,
			IsActive:     true,
		},
		{
			Name:         "Instagram",
			Slug:         "instagram",
			Category:     "social",
			URLPattern:   "https://instagram.com/{value}",
			IconURL:      "https://www.google.com/s2/favicons?domain=instagram.com&sz=128",
			Placeholder:  "username",
			Description:  "Photo and video sharing profile",
			DisplayOrder: 4,
			IsActive:     true,
		},
		{
			Name:         "YouTube",
			Slug:         "youtube",
			Category:     "media",
			URLPattern:   "https://youtube.com/@{value}",
			IconURL:      "https://www.google.com/s2/favicons?domain=youtube.com&sz=128",
			Placeholder:  "channel_handle",
			Description:  "Video channel and streaming profile",
			DisplayOrder: 5,
			IsActive:     true,
		},
		{
			Name:         "WhatsApp",
			Slug:         "whatsapp",
			Category:     "messaging",
			URLPattern:   "https://wa.me/{value}",
			IconURL:      "https://www.google.com/s2/favicons?domain=whatsapp.com&sz=128",
			Placeholder:  "+905550000000 (country code + number)",
			Description:  "Direct WhatsApp chat and instant messaging",
			DisplayOrder: 6,
			IsActive:     true,
		},
		{
			Name:         "Telegram",
			Slug:         "telegram",
			Category:     "messaging",
			URLPattern:   "https://t.me/{value}",
			IconURL:      "https://www.google.com/s2/favicons?domain=telegram.org&sz=128",
			Placeholder:  "username",
			Description:  "Telegram channel or direct contact username",
			DisplayOrder: 7,
			IsActive:     true,
		},
		{
			Name:         "Facebook",
			Slug:         "facebook",
			Category:     "social",
			URLPattern:   "https://facebook.com/{value}",
			IconURL:      "https://www.google.com/s2/favicons?domain=facebook.com&sz=128",
			Placeholder:  "username or page-id",
			Description:  "Personal profile or business page",
			DisplayOrder: 8,
			IsActive:     true,
		},
		{
			Name:         "TikTok",
			Slug:         "tiktok",
			Category:     "media",
			URLPattern:   "https://tiktok.com/@{value}",
			IconURL:      "https://www.google.com/s2/favicons?domain=tiktok.com&sz=128",
			Placeholder:  "username",
			Description:  "Short-form video profile",
			DisplayOrder: 9,
			IsActive:     true,
		},
		{
			Name:         "Threads",
			Slug:         "threads",
			Category:     "social",
			URLPattern:   "https://threads.net/@{value}",
			IconURL:      "https://www.google.com/s2/favicons?domain=threads.net&sz=128",
			Placeholder:  "username",
			Description:  "Text-based conversations connected with Instagram",
			DisplayOrder: 10,
			IsActive:     true,
		},
		{
			Name:         "Discord",
			Slug:         "discord",
			Category:     "community",
			URLPattern:   "https://discord.gg/{value}",
			IconURL:      "https://www.google.com/s2/favicons?domain=discord.com&sz=128",
			Placeholder:  "invite-code or username",
			Description:  "Discord server invite or direct username",
			DisplayOrder: 11,
			IsActive:     true,
		},
		{
			Name:         "Medium",
			Slug:         "medium",
			Category:     "publishing",
			URLPattern:   "https://medium.com/@{value}",
			IconURL:      "https://www.google.com/s2/favicons?domain=medium.com&sz=128",
			Placeholder:  "username",
			Description:  "Articles and publication blog",
			DisplayOrder: 12,
			IsActive:     true,
		},
		{
			Name:         "Behance",
			Slug:         "behance",
			Category:     "portfolio",
			URLPattern:   "https://behance.net/{value}",
			IconURL:      "https://www.google.com/s2/favicons?domain=behance.net&sz=128",
			Placeholder:  "username",
			Description:  "Design showcase and creative portfolio",
			DisplayOrder: 13,
			IsActive:     true,
		},
		{
			Name:         "Dribbble",
			Slug:         "dribbble",
			Category:     "portfolio",
			URLPattern:   "https://dribbble.com/{value}",
			IconURL:      "https://www.google.com/s2/favicons?domain=dribbble.com&sz=128",
			Placeholder:  "username",
			Description:  "UI/UX and visual design portfolio",
			DisplayOrder: 14,
			IsActive:     true,
		},
		{
			Name:         "Spotify",
			Slug:         "spotify",
			Category:     "media",
			URLPattern:   "https://open.spotify.com/user/{value}",
			IconURL:      "https://www.google.com/s2/favicons?domain=spotify.com&sz=128",
			Placeholder:  "user-id or artist-id",
			Description:  "Music artist profile or playlist",
			DisplayOrder: 15,
			IsActive:     true,
		},
		{
			Name:         "Personal Website / Portfolio",
			Slug:         "website",
			Category:     "general",
			URLPattern:   "https://{value}",
			IconURL:      "https://www.google.com/s2/favicons?domain=google.com&sz=128",
			Placeholder:  "example.com",
			Description:  "Personal portfolio or company website",
			DisplayOrder: 16,
			IsActive:     true,
		},
	}

	query := `
		INSERT INTO social_platforms (
			name, slug, category, url_pattern, icon_url, placeholder, description, display_order, is_active
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9
		)
		ON CONFLICT (slug) WHERE deleted_at IS NULL DO UPDATE
		SET name = EXCLUDED.name,
			category = EXCLUDED.category,
			url_pattern = EXCLUDED.url_pattern,
			icon_url = EXCLUDED.icon_url,
			placeholder = EXCLUDED.placeholder,
			description = EXCLUDED.description,
			display_order = EXCLUDED.display_order,
			is_active = EXCLUDED.is_active,
			updated_at = NOW();
	`

	for _, p := range platforms {
		_, err := pool.Exec(ctx, query,
			p.Name,
			p.Slug,
			p.Category,
			p.URLPattern,
			p.IconURL,
			p.Placeholder,
			p.Description,
			int32(p.DisplayOrder),
			p.IsActive,
		)
		if err != nil {
			return fmt.Errorf("failed to upsert platform %s: %w", p.Slug, err)
		}
	}

	return nil
}

func seedSubscriberUser(ctx context.Context, pool *pgxpool.Pool) error {
	email := "gamefurkanmeclis@gmail.com"
	name := "Furkan"
	surname := "Meclis"
	pass := "Password1!"

	hash, err := password.Hash(pass)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	var userID int64
	upsertUserSQL := `
		INSERT INTO users (
			email, password_hash, name, surname, status, email_verified_at, locale
		) VALUES (
			$1, $2, $3, $4, 'active', NOW(), 'tr'
		)
		ON CONFLICT (email) WHERE deleted_at IS NULL DO UPDATE
		SET name = EXCLUDED.name,
			surname = EXCLUDED.surname,
			status = 'active',
			locale = 'tr',
			updated_at = NOW()
		RETURNING id;
	`
	err = pool.QueryRow(ctx, upsertUserSQL, email, hash, name, surname).Scan(&userID)
	if err != nil {
		return fmt.Errorf("upsert user: %w", err)
	}

	// Assign subscriber role
	roleSQL := `
		INSERT INTO user_roles (user_id, role_id)
		SELECT $1, r.id FROM roles r
		WHERE r.slug = 'subscriber'
		ON CONFLICT (user_id, role_id) DO NOTHING;
	`
	_, err = pool.Exec(ctx, roleSQL, userID)
	if err != nil {
		return fmt.Errorf("assign subscriber role: %w", err)
	}

	fmt.Printf("   User: %s (ID: %d) successfully seeded with subscriber role.\n", email, userID)
	return nil
}

func seedSubscriberProfiles(ctx context.Context, pool *pgxpool.Pool) error {
	var userID int64
	err := pool.QueryRow(ctx, `SELECT id FROM users WHERE email = 'gamefurkanmeclis@gmail.com' AND deleted_at IS NULL`).Scan(&userID)
	if err != nil {
		return fmt.Errorf("could not find subscriber user gamefurkanmeclis@gmail.com: %w", err)
	}
	// Profiles belong to accounts; the demo profiles go to the user's personal account.
	var accountID int64
	err = pool.QueryRow(ctx, `SELECT id FROM accounts WHERE personal_user_id = $1`, userID).Scan(&accountID)
	if err != nil {
		return fmt.Errorf("could not find personal account of subscriber user: %w", err)
	}

	// Platform map for foreign keys
	platformMap := make(map[string]int64)
	rows, err := pool.Query(ctx, `SELECT id, slug FROM social_platforms WHERE deleted_at IS NULL`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var pid int64
			var slug string
			if err := rows.Scan(&pid, &slug); err == nil {
				platformMap[slug] = pid
			}
		}
	}

	// 1. Clear existing demo profiles for clean regeneration
	_, _ = pool.Exec(ctx, `DELETE FROM profiles WHERE account_id = $1`, accountID)

	// Profile 1: Executive & CTO Profile (Primary, TR + EN)
	p1UUID := uuid.New()
	var p1ID int64
	err = pool.QueryRow(ctx, `
		INSERT INTO profiles (
			uuid, account_id, user_id, primary_locale, is_primary, completion_rate, birthday, gender,
			avatar_url, cover_url
		) VALUES (
			$1, $3, $2, 'tr', true, 100, '1995-06-15', 'male',
			'https://images.unsplash.com/photo-1534528741775-53994a69daeb?auto=format&fit=crop&w=400&q=80',
			'https://images.unsplash.com/photo-1618005182384-a83a8bd57fbe?auto=format&fit=crop&w=1200&q=80'
		) RETURNING id;
	`, p1UUID, userID, accountID).Scan(&p1ID)
	if err != nil {
		return fmt.Errorf("create profile 1: %w", err)
	}

	// P1 Translations (TR & EN)
	_, err = pool.Exec(ctx, `
		INSERT INTO profile_translations (
			profile_id, locale, prefix, first_name, last_name, middle_name, suffix, nickname,
			company_name, department, title, role, bio, note, is_default
		) VALUES 
		($1, 'tr', 'Dr.', 'Furkan', 'Meclis', 'Can', 'MBA', 'Chief',
		 'ComCode Teknoloji A.Ş.', 'Mühendislik & Ar-Ge', 'Chief Technology Officer (CTO)', 'Bulut Mimarisi & Dağıtık Sistemler',
		 'Yüksek ölçekli bulut sistemleri, mikroservis mimarileri ve modern web teknolojileri üzerine uzmanlaşmış yazılım mühendisi ve sistem mimarı.',
		 'Resmi iş iletişimi ve teknik ortaklıklar için öncelikli kanallardan ulaşabilirsiniz.', true),
		($1, 'en', 'Dr.', 'Furkan', 'Meclis', 'Can', 'MBA', 'Chief',
		 'ComCode Software Inc.', 'Engineering & R&D', 'Chief Technology Officer (CTO)', 'Cloud Architecture & Distributed Systems',
		 'Software engineer and system architect specializing in high-scale cloud platforms, distributed systems, and modern full-stack web applications.',
		 'Feel free to connect directly for business collaborations and technical inquiries.', false);
	`, p1ID)
	if err != nil {
		return fmt.Errorf("create profile 1 translations: %w", err)
	}

	// P1 Contact Details
	_, err = pool.Exec(ctx, `
		INSERT INTO profile_contact_details (
			profile_id, category, type, label, value, is_primary, display_order
		) VALUES
		($1, 'phone', 'mobile', 'Şahsi / Direct Line', '+90 532 123 45 67', true, 1),
		($1, 'phone', 'work', 'Genel Merkez Santral', '+90 212 987 65 43', false, 2),
		($1, 'email', 'work', 'Kurumsal E-posta', 'furkan@comcode.app', false, 3),
		($1, 'email', 'home', 'Kişisel İletişim', 'gamefurkanmeclis@gmail.com', false, 4),
		($1, 'whatsapp', 'mobile', 'WhatsApp Business', '+90 532 123 45 67', false, 5),
		($1, 'url', 'work', 'Kişisel Portfolyo', 'https://furkanmeclis.com', false, 6);
	`, p1ID)
	if err != nil {
		return fmt.Errorf("create profile 1 contacts: %w", err)
	}

	// P1 Addresses
	_, err = pool.Exec(ctx, `
		INSERT INTO profile_addresses (
			profile_id, type, label, street, extended, city, region, postal_code, country, is_primary, display_order
		) VALUES
		($1, 'work', 'Genel Merkez Ofisi', 'Büyükdere Cad. No: 199 Levent 199', 'Kat: 18 D: 1804', 'İstanbul', 'Şişli', '34394', 'Türkiye', true, 1),
		($1, 'other', 'US Innovation Hub', '100 Pine Street, Suite 1250', 'Floor 12', 'San Francisco', 'CA', '94111', 'United States', false, 2);
	`, p1ID)
	if err != nil {
		return fmt.Errorf("create profile 1 addresses: %w", err)
	}

	// P1 Social Links
	_, err = pool.Exec(ctx, `
		INSERT INTO profile_social_links (
			profile_id, platform_id, platform, label, url, icon_url, display_order, is_active
		) VALUES
		($1, $2, 'linkedin', 'LinkedIn', 'https://linkedin.com/in/furkanmeclis', 'https://www.google.com/s2/favicons?domain=linkedin.com&sz=128', 1, true),
		($1, $3, 'github', 'GitHub', 'https://github.com/furkanmeclis', 'https://www.google.com/s2/favicons?domain=github.com&sz=128', 2, true),
		($1, $4, 'twitter', 'X (Twitter)', 'https://x.com/furkanmeclis', 'https://www.google.com/s2/favicons?domain=x.com&sz=128', 3, true),
		($1, $5, 'youtube', 'ComCode Dev', 'https://youtube.com/@comcode', 'https://www.google.com/s2/favicons?domain=youtube.com&sz=128', 4, true),
		($1, $6, 'medium', 'Teknoloji Yazıları', 'https://medium.com/@furkanmeclis', 'https://www.google.com/s2/favicons?domain=medium.com&sz=128', 5, true);
	`, p1ID, platformMap["linkedin"], platformMap["github"], platformMap["twitter"], platformMap["youtube"], platformMap["medium"])
	if err != nil {
		return fmt.Errorf("create profile 1 social links: %w", err)
	}

	// Profile 2: Senior Go & Cloud Architect (Developer Profile)
	p2UUID := uuid.New()
	var p2ID int64
	err = pool.QueryRow(ctx, `
		INSERT INTO profiles (
			uuid, account_id, user_id, primary_locale, is_primary, completion_rate,
			avatar_url, cover_url
		) VALUES (
			$1, $3, $2, 'en', false, 90,
			'https://images.unsplash.com/photo-1507003211169-0a1dd7228f2d?auto=format&fit=crop&w=400&q=80',
			'https://images.unsplash.com/photo-1526374965328-7f61d4dc18c5?auto=format&fit=crop&w=1200&q=80'
		) RETURNING id;
	`, p2UUID, userID, accountID).Scan(&p2ID)
	if err != nil {
		return fmt.Errorf("create profile 2: %w", err)
	}

	// P2 Translations
	_, err = pool.Exec(ctx, `
		INSERT INTO profile_translations (
			profile_id, locale, first_name, last_name, company_name, department, title, role, bio, note, is_default
		) VALUES
		($1, 'en', 'Furkan', 'Meclis', 'ComCode Labs', 'Open Source Engineering', 'Senior Go & Distributed Systems Architect', 'High-Throughput Services & K8s',
		 'Building ultra-resilient distributed services with Golang, gRPC, and PostgreSQL.',
		 'Reach out via GitHub or developer email for open-source contributions and technical consulting.', true),
		($1, 'tr', 'Furkan', 'Meclis', 'ComCode Labs', 'Açık Kaynak Geliştirme', 'Kıdemli Go & Dağıtık Sistemler Mimarı', 'Yüksek Verimli Servisler & K8s',
		 'Golang, gRPC ve PostgreSQL ile yüksek ölçeklenebilir arka uç altyapıları geliştiricisi.',
		 'Açık kaynak projeler ve teknik danışmanlık talepleriniz için iletişime geçebilirsiniz.', false);
	`, p2ID)
	if err != nil {
		return fmt.Errorf("create profile 2 translations: %w", err)
	}

	// P2 Contacts
	_, err = pool.Exec(ctx, `
		INSERT INTO profile_contact_details (
			profile_id, category, type, label, value, is_primary, display_order
		) VALUES
		($1, 'phone', 'mobile', 'Geliştirici Hattı', '+90 555 987 65 43', true, 1),
		($1, 'email', 'work', 'Dev Team', 'dev@comcode.app', false, 2);
	`, p2ID)
	if err != nil {
		return fmt.Errorf("create profile 2 contacts: %w", err)
	}

	// P2 Addresses
	_, err = pool.Exec(ctx, `
		INSERT INTO profile_addresses (
			profile_id, type, label, street, extended, city, region, postal_code, country, is_primary, display_order
		) VALUES
		($1, 'work', 'Ar-Ge Laboratuvarı', 'İTÜ Arı Teknokent 3, No: 4', 'Bina B Blok K: 2', 'İstanbul', 'Sarıyer', '34469', 'Türkiye', true, 1);
	`, p2ID)
	if err != nil {
		return fmt.Errorf("create profile 2 addresses: %w", err)
	}

	// P2 Social Links
	_, err = pool.Exec(ctx, `
		INSERT INTO profile_social_links (
			profile_id, platform_id, platform, label, url, icon_url, display_order, is_active
		) VALUES
		($1, $2, 'github', 'GitHub Profile', 'https://github.com/furkanmeclis', 'https://www.google.com/s2/favicons?domain=github.com&sz=128', 1, true),
		($1, $3, 'discord', 'Dev Discord Community', 'https://discord.gg/comcode', 'https://www.google.com/s2/favicons?domain=discord.com&sz=128', 2, true),
		($1, $4, 'medium', 'Technical Articles', 'https://medium.com/@furkanmeclis', 'https://www.google.com/s2/favicons?domain=medium.com&sz=128', 3, true);
	`, p2ID, platformMap["github"], platformMap["discord"], platformMap["medium"])
	if err != nil {
		return fmt.Errorf("create profile 2 social links: %w", err)
	}

	// Profile 3: Creative & Gaming Community Profile
	p3UUID := uuid.New()
	var p3ID int64
	err = pool.QueryRow(ctx, `
		INSERT INTO profiles (
			uuid, account_id, user_id, primary_locale, is_primary, completion_rate,
			avatar_url, cover_url
		) VALUES (
			$1, $3, $2, 'tr', false, 85,
			'https://images.unsplash.com/photo-1566492031773-4f4e44671857?auto=format&fit=crop&w=400&q=80',
			'https://images.unsplash.com/photo-1550745165-9bc0b252726f?auto=format&fit=crop&w=1200&q=80'
		) RETURNING id;
	`, p3UUID, userID, accountID).Scan(&p3ID)
	if err != nil {
		return fmt.Errorf("create profile 3: %w", err)
	}

	// P3 Translations
	_, err = pool.Exec(ctx, `
		INSERT INTO profile_translations (
			profile_id, locale, first_name, last_name, nickname, company_name, title, role, bio, note, is_default
		) VALUES
		($1, 'tr', 'Furkan', 'Meclis', 'GameDev', 'Indie Studio', 'Bağımsız Oyun Geliştiricisi & Yayıncı', 'Unreal Engine & Unity',
		 'Oyun geliştirme, canlı yayınlar ve interaktif dijital deneyimler üreticisi.',
		 'Sponsorluk ve topluluk iş birlikleri için lütfen e-posta üzerinden ulaşın.', true),
		($1, 'en', 'Furkan', 'Meclis', 'GameDev', 'Indie Studio', 'Indie Game Developer & Streamer', 'Unreal Engine & Unity',
		 'Indie game development, interactive storytelling, and community gaming streams.',
		 'For sponsorships and community inquiries, reach out via email.', false);
	`, p3ID)
	if err != nil {
		return fmt.Errorf("create profile 3 translations: %w", err)
	}

	// P3 Contacts
	_, err = pool.Exec(ctx, `
		INSERT INTO profile_contact_details (
			profile_id, category, type, label, value, is_primary, display_order
		) VALUES
		($1, 'email', 'other', 'Sponsorluk & İletişim', 'gamefurkanmeclis@gmail.com', true, 1),
		($1, 'whatsapp', 'mobile', 'Menajerlik Hattı', '+90 532 123 45 67', false, 2);
	`, p3ID)
	if err != nil {
		return fmt.Errorf("create profile 3 contacts: %w", err)
	}

	// P3 Addresses
	_, err = pool.Exec(ctx, `
		INSERT INTO profile_addresses (
			profile_id, type, label, street, extended, city, region, postal_code, country, is_primary, display_order
		) VALUES
		($1, 'home', 'Yaratıcı Stüdyo & Yayın Odası', 'Moda Cad. No: 42', 'Daire: 5', 'İstanbul', 'Kadıköy', '34710', 'Türkiye', true, 1);
	`, p3ID)
	if err != nil {
		return fmt.Errorf("create profile 3 addresses: %w", err)
	}

	// P3 Social Links
	_, err = pool.Exec(ctx, `
		INSERT INTO profile_social_links (
			profile_id, platform_id, platform, label, url, icon_url, display_order, is_active
		) VALUES
		($1, $2, 'youtube', 'YouTube Gaming', 'https://youtube.com/@gamefurkan', 'https://www.google.com/s2/favicons?domain=youtube.com&sz=128', 1, true),
		($1, $3, 'discord', 'Oyun Topluluğu', 'https://discord.gg/gamedev', 'https://www.google.com/s2/favicons?domain=discord.com&sz=128', 2, true),
		($1, $4, 'spotify', 'Yayın Çalma Listeleri', 'https://open.spotify.com/user/furkanmeclis', 'https://www.google.com/s2/favicons?domain=spotify.com&sz=128', 3, true),
		($1, $5, 'instagram', 'Instagram', 'https://instagram.com/furkanmeclis', 'https://www.google.com/s2/favicons?domain=instagram.com&sz=128', 4, true);
	`, p3ID, platformMap["youtube"], platformMap["discord"], platformMap["spotify"], platformMap["instagram"])
	if err != nil {
		return fmt.Errorf("create profile 3 social links: %w", err)
	}

	fmt.Printf("   Generated 3 comprehensive digital vCard profiles for %s (Primary CTO card, Developer card, Gaming card)\n", "gamefurkanmeclis@gmail.com")
	return nil
}

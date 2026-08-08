package webhook

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/sunnyhmz7010/CList/internal/storage"
)

var (
	ErrInvalidSecret = errors.New("invalid webhook secret")
	ErrInvalidUpdate = errors.New("invalid Telegram webhook update")
	ErrChannel       = errors.New("Telegram channel does not match storage profile")
)

type Sender interface {
	SendMessage(ctx context.Context, chatID, text string, replyTo int64) error
}

type Profile struct {
	ID            string
	ChannelID     string
	Secret        string
	PublicBaseURL string
	Sender        Sender
}

type Resolver interface {
	Resolve(ctx context.Context, publicSecret string) (Profile, error)
}

type StorageResolver struct {
	profiles  *storage.ProfileService
	newSender func(baseURL, token string) Sender
}

func NewStorageResolver(profiles *storage.ProfileService, newSender func(baseURL, token string) Sender) *StorageResolver {
	return &StorageResolver{profiles: profiles, newSender: newSender}
}

func (r *StorageResolver) Resolve(ctx context.Context, publicSecret string) (Profile, error) {
	profiles, err := r.profiles.List(ctx)
	if err != nil {
		return Profile{}, err
	}
	for _, item := range profiles {
		if item.Type != "telegram_official" && item.Type != "telegram_streaming" {
			continue
		}
		config, err := r.profiles.Config(ctx, item.ID)
		if err != nil {
			return Profile{}, err
		}
		pathSecret := config["webhook_public_secret"]
		if pathSecret == "" {
			pathSecret = config["webhook_secret"]
		}
		if pathSecret != publicSecret {
			continue
		}
		if r.newSender == nil {
			return Profile{}, errors.New("未配置 Telegram 回复客户端工厂")
		}
		return Profile{ID: item.ID, ChannelID: config["channel_id"], Secret: config["webhook_secret"], PublicBaseURL: config["public_base_url"], Sender: r.newSender(config["base_url"], config["bot_token"])}, nil
	}
	return Profile{}, storage.ErrBackendNotFound
}

type Service struct {
	db       *sql.DB
	profiles Resolver
}

func NewService(database *sql.DB, profiles Resolver) *Service {
	return &Service{db: database, profiles: profiles}
}

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

type Update struct {
	ChannelPost *ChannelPost `json:"channel_post"`
}

type ChannelPost struct {
	MessageID int64 `json:"message_id"`
	Chat      struct {
		ID int64 `json:"id"`
	} `json:"chat"`
	Document *Document `json:"document,omitempty"`
	Video    *Document `json:"video,omitempty"`
	Audio    *Document `json:"audio,omitempty"`
}

type Document struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	FileSize     int64  `json:"file_size"`
	FileName     string `json:"file_name"`
	MIMEType     string `json:"mime_type"`
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	publicSecret := strings.TrimPrefix(r.URL.Path, "/webhooks/telegram/")
	profile, err := h.service.profiles.Resolve(r.Context(), publicSecret)
	if err != nil || subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Telegram-Bot-Api-Secret-Token")), []byte(profile.Secret)) != 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var update Update
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&update); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if err := h.service.Ingest(r.Context(), profile, update); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Service) Ingest(ctx context.Context, profile Profile, update Update) error {
	if update.ChannelPost == nil || update.ChannelPost.MessageID == 0 {
		return ErrInvalidUpdate
	}
	post := update.ChannelPost
	if strconv.FormatInt(post.Chat.ID, 10) != profile.ChannelID {
		return ErrChannel
	}
	document := post.Document
	if document == nil {
		document = post.Video
	}
	if document == nil {
		document = post.Audio
	}
	if document == nil || document.FileID == "" || document.FileUniqueID == "" || document.FileName == "" || document.FileSize < 0 {
		return ErrInvalidUpdate
	}
	if document.MIMEType == "" {
		document.MIMEType = "application/octet-stream"
	}
	publicID, created, err := s.insert(ctx, profile, post, document)
	if err != nil || !created {
		return err
	}
	link := "/f/" + publicID + "/" + url.PathEscape(document.FileName)
	if profile.PublicBaseURL != "" {
		link = strings.TrimRight(profile.PublicBaseURL, "/") + link
	}
	if profile.Sender == nil {
		return errors.New("未配置 Telegram 回复客户端")
	}
	return profile.Sender.SendMessage(ctx, profile.ChannelID, link, post.MessageID)
}

func (s *Service) insert(ctx context.Context, profile Profile, post *ChannelPost, document *Document) (string, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", false, err
	}
	defer tx.Rollback()
	var existing sql.NullString
	err = tx.QueryRowContext(ctx, "SELECT file_public_id FROM telegram_messages WHERE chat_id=? AND message_id=?", profile.ChannelID, post.MessageID).Scan(&existing)
	if err == nil {
		return existing.String, false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", false, err
	}
	publicID, err := randomID()
	if err != nil {
		return "", false, err
	}
	ref := storage.TelegramRef{ChatID: profile.ChannelID, MessageID: post.MessageID, FileID: document.FileID, FileUniqueID: document.FileUniqueID, Size: document.FileSize}
	key, err := encodeRef(ref)
	if err != nil {
		return "", false, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = tx.ExecContext(ctx, `INSERT INTO files(public_id,storage_profile_id,storage_key,file_name,mime_type,size,sha256,state,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?)`,
		publicID, profile.ID, key, document.FileName, document.MIMEType, document.FileSize, "", "active", now, now)
	if err != nil {
		return "", false, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO telegram_messages(chat_id,message_id,storage_profile_id,file_public_id,file_id,file_unique_id,file_size,created_at) VALUES(?,?,?,?,?,?,?,?)`,
		profile.ChannelID, post.MessageID, profile.ID, publicID, document.FileID, document.FileUniqueID, document.FileSize, now)
	if err != nil {
		return "", false, err
	}
	if err := tx.Commit(); err != nil {
		return "", false, err
	}
	return publicID, true, nil
}

func encodeRef(ref storage.TelegramRef) (string, error) {
	raw, err := json.Marshal(ref)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func randomID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

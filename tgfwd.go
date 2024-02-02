package tgfwd

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/telegram/downloader"
	"github.com/gotd/td/telegram/message"
	"github.com/gotd/td/telegram/message/html"
	"github.com/gotd/td/telegram/message/peer"
	"github.com/gotd/td/telegram/uploader"
	"github.com/gotd/td/tg"
)

type Config struct {
	Phone       string
	ID          int
	Hash        string
	SessionPath string
	From        []int64
	To          int64
	ToType      string
	Forwards    [][2]int64
	Debug       bool
}

func Login(ctx context.Context, cfg *Config) error {
	if cfg.Phone == "" {
		return fmt.Errorf("tgfwd: phone is required")
	}
	if cfg.ID == 0 {
		return fmt.Errorf("tgfwd: app id is required")
	}
	if cfg.Hash == "" {
		return fmt.Errorf("tgfwd: hash is required")
	}
	if cfg.SessionPath == "" {
		return fmt.Errorf("tgfwd: session path is required")
	}

	// Obtain code from stdin.
	codePrompt := func(ctx context.Context, sentCode *tg.AuthSentCode) (string, error) {
		fmt.Print("Enter code: ")
		code, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("tgfwd: couldn't read code: %w", err)
		}
		return strings.TrimSpace(code), nil
	}

	client := telegram.NewClient(cfg.ID, cfg.Hash, telegram.Options{
		SessionStorage: &session.FileStorage{
			Path: cfg.SessionPath,
		},
	})

	// This will setup and perform authentication flow.
	flow := auth.NewFlow(
		auth.CodeOnly(cfg.Phone, auth.CodeAuthenticatorFunc(codePrompt)),
		auth.SendCodeOptions{},
	)

	return client.Run(ctx, func(ctx context.Context) error {
		if err := client.Auth().IfNecessary(ctx, flow); err != nil {
			return fmt.Errorf("tgfwd: couldn't auth: %w", err)
		}
		fmt.Println("tgfwd: logged in")
		return nil
	})
}

func List(ctx context.Context, cfg *Config) error {
	if cfg.ID == 0 {
		return fmt.Errorf("tgfwd: app id is required")
	}
	if cfg.Hash == "" {
		return fmt.Errorf("tgfwd: hash is required")
	}
	if cfg.SessionPath == "" {
		return fmt.Errorf("tgfwd: session path is required")
	}

	// Create client
	client := telegram.NewClient(cfg.ID, cfg.Hash, telegram.Options{
		SessionStorage: &session.FileStorage{
			Path: cfg.SessionPath,
		},
	})

	// Raw MTProto API client, allows making raw RPC calls
	api := tg.NewClient(client)

	return client.Run(ctx, func(ctx context.Context) error {
		status, err := client.Auth().Status(ctx)
		if err != nil {
			return fmt.Errorf("tgfwd: couldn't get auth status: %w", err)
		}
		if !status.Authorized {
			return fmt.Errorf("tgfwd: not authorized")
		}

		// Get dialogs
		getDialogs, err := api.MessagesGetDialogs(ctx, &tg.MessagesGetDialogsRequest{
			Limit:      100,
			OffsetPeer: &tg.InputPeerEmpty{},
		})
		if err != nil {
			return fmt.Errorf("tgfwd: couldn't get dialogs: %w", err)
		}
		dialogs, ok := getDialogs.AsModified()
		if !ok {
			return fmt.Errorf("tgfwd: unexpected dialogs type: %T", dialogs)
		}
		var chats []tg.FullChat
		var channels []tg.FullChat
		for _, c := range dialogs.GetChats() {
			c, ok := c.AsFull()
			if !ok {
				continue
			}
			if c.TypeName() == "channel" {
				channels = append(channels, c)
			} else {
				chats = append(chats, c)
			}
		}

		for _, c := range channels {
			fmt.Printf("%s %d: %s\n", c.TypeName(), c.GetID(), c.GetTitle())
		}
		for _, c := range chats {
			fmt.Printf("%s %d: %s\n", c.TypeName(), c.GetID(), c.GetTitle())
		}

		// Get users
		getContacts, err := api.ContactsGetContacts(ctx, 0)
		if err != nil {
			return fmt.Errorf("tgfwd: couldn't get users: %w", err)
		}
		contacts, ok := getContacts.AsModified()
		if !ok {
			return fmt.Errorf("tgfwd: unexpected users type: %T", contacts)
		}
		for _, u := range contacts.GetUsers() {
			u, ok := u.AsNotEmpty()
			if !ok {
				continue
			}
			fmt.Printf("user %d: %s %s (%s)\n", u.GetID(), u.FirstName, u.LastName, u.Username)
		}
		return nil
	})
}

func Run(ctx context.Context, cfg *Config) error {
	if cfg.ID == 0 {
		return fmt.Errorf("tgfwd: app id is required")
	}
	if cfg.Hash == "" {
		return fmt.Errorf("tgfwd: hash is required")
	}
	if cfg.SessionPath == "" {
		return fmt.Errorf("tgfwd: session path is required")
	}
	if len(cfg.Forwards) == 0 {
		return fmt.Errorf("tgfwd: at least one forward is required")
	}

	debug := func(string, ...any) {}
	if cfg.Debug {
		debug = log.Printf
	}

	dispatcher := tg.NewUpdateDispatcher()

	client := telegram.NewClient(cfg.ID, cfg.Hash, telegram.Options{
		SessionStorage: &session.FileStorage{
			Path: cfg.SessionPath,
		},
		UpdateHandler: dispatcher,
	})

	// Raw MTProto API client, allows making raw RPC calls
	api := tg.NewClient(client)

	// Helper for sending messages
	sender := message.NewSender(api)

	// Forward lookup
	forwards := map[int64]tg.InputPeerClass{}
	// Chat lookup
	chats := make(map[int64]tg.FullChat)

	download := downloader.NewDownloader()
	upload := uploader.NewUploader(api)

	onMessage := func(ctx context.Context, m *tg.Message) error {
		// Obtain peer ID
		fromID, err := fromPeer(m.PeerID)
		if err != nil {
			log.Println(fmt.Errorf("tgfwd: couldn't get peer id: %w", err))
			return nil
		}

		// TODO: process other types of messages (photos, videos, etc.)
		media, err := downloadMedia(ctx, api, download, m)
		if err != nil {
			log.Println(fmt.Errorf("tgfwd: couldn't get media: %w", err))
		}

		// Check if message is empty
		if m.Message == "" && len(media) == 0 {
			js, _ := json.MarshalIndent(m, "", "  ")
			debug("tgfwd: empty message: %s", js)
			return nil
		}

		// Check if message is forwarded from a target peer
		to, ok := forwards[fromID]
		if !ok {
			return nil
		}
		toID := fromInputPeer(to)
		debug("tgfwd: forwarded message from %d (%s) to %d (%s)", fromID, chats[fromID].GetTitle(), toID, chats[toID].GetTitle())

		// Forward media
		if len(media) > 0 {
			// Forward media to target peer
			inputFile, err := upload.FromBytes(ctx, "", media)
			if err != nil {
				log.Println(fmt.Errorf("tgfwd: couldn't upload media: %w", err))
			}
			mediaOpt := message.Media(&tg.InputMediaUploadedPhoto{
				File: inputFile,
			}, html.String(nil, m.Message))
			if _, err := sender.To(to).Media(ctx, mediaOpt); err != nil {
				log.Println(fmt.Errorf("tgfwd: couldn't forward media: %w", err))
				return nil
			}
			return nil
		}

		// Forward only text
		if _, err := sender.To(to).Text(ctx, m.Message); err != nil {
			log.Println(fmt.Errorf("tgfwd: couldn't forward message: %w", err))
			return nil
		}

		return nil
	}

	return client.Run(ctx, func(ctx context.Context) error {
		status, err := client.Auth().Status(ctx)
		if err != nil {
			return fmt.Errorf("tgfwd: couldn't get auth status: %w", err)
		}
		if !status.Authorized {
			return fmt.Errorf("tgfwd: not authorized")
		}

		// List dialogs
		getDialogs, err := api.MessagesGetDialogs(ctx, &tg.MessagesGetDialogsRequest{
			Limit:      100,
			OffsetPeer: &tg.InputPeerEmpty{},
		})
		if err != nil {
			return fmt.Errorf("tgfwd: couldn't get dialogs: %w", err)
		}
		dialogs, ok := getDialogs.AsModified()
		if !ok {
			return fmt.Errorf("tgfwd: unexpected dialogs type: %T", dialogs)
		}
		for _, c := range dialogs.GetChats() {
			c, ok := c.AsFull()
			if !ok {
				continue
			}
			chats[c.GetID()] = c
		}

		// Generate forwards
		peers := map[int64]tg.InputPeerClass{}
		for _, f := range cfg.Forwards {
			// Normalize IDs
			fromID, toID := f[0], f[1]
			if fromID < 0 {
				fromID = -fromID
			}
			if toID < 0 {
				toID = -toID
			}

			// Check if from exists
			fromChat, ok := chats[fromID]
			if !ok {
				return fmt.Errorf("tgfwd: couldn't find chat %d", fromID)
			}

			// Check if to exists
			toChat, ok := chats[toID]
			if !ok {
				return fmt.Errorf("tgfwd: couldn't find chat %d", toID)
			}

			// Check if we already have a peer
			peer, ok := peers[toID]
			if !ok {
				// Get peer
				peer, err = toInputPeer(ctx, toChat)
				if err != nil {
					return err
				}
				peers[toID] = peer
			}

			// Save peer
			forwards[fromID] = peer
			debug("tgfwd: forwarding %d (%s) to %d (%s)", fromID, fromChat.GetTitle(), toID, toChat.GetTitle())
		}

		// Setting up handler for incoming message
		dispatcher.OnNewMessage(func(ctx context.Context, entities tg.Entities, u *tg.UpdateNewMessage) error {
			m, ok := u.Message.(*tg.Message)
			if !ok /*|| m.Out*/ {
				// Outgoing message, not interesting
				return nil
			}
			return onMessage(ctx, m)
		})

		// Setting up handler for incoming channel message
		dispatcher.OnNewChannelMessage(func(ctx context.Context, entities tg.Entities, u *tg.UpdateNewChannelMessage) error {
			m, ok := u.Message.(*tg.Message)
			if !ok || m.Out {
				// Outgoing message, not interesting
				return nil
			}
			return onMessage(ctx, m)
		})

		log.Println("tgfwd: started")
		<-ctx.Done()
		log.Println("tgfwd: stopped")
		return nil
	})
}

func fromPeer(p tg.PeerClass) (id int64, err error) {
	switch v := p.(type) {
	case *tg.PeerUser:
		return v.UserID, nil
	case *tg.PeerChannel:
		return v.ChannelID, nil
	case *tg.PeerChat:
		return v.ChatID, nil
	}
	return 0, fmt.Errorf("invalid peer: %T", p)
}

func toInputPeer(ctx context.Context, chat tg.FullChat) (p tg.InputPeerClass, err error) {
	switch chat.TypeName() {
	case "user":
		return peer.OnlyUser(func(ctx context.Context) (tg.InputPeerClass, error) {
			return &tg.InputPeerUser{
				UserID: chat.GetID(),
			}, nil
		})(ctx)
	case "chat":
		return peer.OnlyChat(func(ctx context.Context) (tg.InputPeerClass, error) {
			return &tg.InputPeerChat{
				ChatID: chat.GetID(),
			}, nil
		})(ctx)
	case "channel":
		return peer.OnlyChannel(func(ctx context.Context) (tg.InputPeerClass, error) {
			return &tg.InputPeerChannel{
				ChannelID: chat.GetID(),
			}, nil
		})(ctx)
	default:
		return nil, fmt.Errorf("invalid type: %s", chat.TypeName())
	}
}

func fromInputPeer(p tg.InputPeerClass) int64 {
	switch v := p.(type) {
	case *tg.InputPeerUser:
		return v.UserID
	case *tg.InputPeerChannel:
		return v.ChannelID
	case *tg.InputPeerChat:
		return v.ChatID
	default:
		return 0
	}
}

func downloadMedia(ctx context.Context, api *tg.Client, download *downloader.Downloader, m *tg.Message) ([]byte, error) {
	if m.Media != nil {
		switch v := m.Media.(type) {
		case *tg.MessageMediaPhoto:
			photo, ok := v.Photo.AsNotEmpty()
			if !ok {
				return nil, nil
			}
			var size string
			for _, s := range photo.Sizes {
				v, ok := s.(*tg.PhotoSize)
				if !ok {
					continue
				}
				size = v.Type
			}
			if size == "" {
				return nil, fmt.Errorf("couldn't find photo size")
			}
			loc := &tg.InputPhotoFileLocation{
				ID:            photo.ID,
				AccessHash:    photo.AccessHash,
				FileReference: photo.FileReference,
				ThumbSize:     size,
			}
			var buf bytes.Buffer
			b := download.Download(api, loc)
			if _, err := b.Stream(ctx, &buf); err != nil {
				return nil, err
			}
			return buf.Bytes(), nil
		default:
		}
	}
	return nil, nil
}

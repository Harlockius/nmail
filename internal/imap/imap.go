// Package imap provides IMAP client functionality for nmail.
package imap

import (
	"crypto/tls"
	"fmt"
	"io"
	"mime"
	"strings"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-message/charset"
	"github.com/harlock/nmail/internal/config"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/transform"
)

func init() {
	// Register Korean charset decoders for go-message
	charset.RegisterEncoding("euc-kr", korean.EUCKR)
	charset.RegisterEncoding("ks_c_5601-1987", korean.EUCKR)
}

// Message represents a parsed email message.
type Message struct {
	ID      uint32    `json:"id"`
	From    string    `json:"from"`
	Subject string    `json:"subject"`
	Date    time.Time `json:"date"`
	Body    string    `json:"body,omitempty"`
	IsRead  bool      `json:"is_read"`
}

// Client wraps an IMAP connection.
type Client struct {
	c *imapclient.Client
}

// Connect opens a TLS IMAP connection and logs in.
func Connect(account config.Account) (*Client, error) {
	addr := fmt.Sprintf("%s:%d", account.IMAPHost, account.IMAPPort)

	var c *imapclient.Client
	var err error

	if account.IMAPTLS {
		c, err = imapclient.DialTLS(addr, &imapclient.Options{
			TLSConfig: &tls.Config{ServerName: account.IMAPHost},
		})
	} else {
		c, err = imapclient.DialInsecure(addr, nil)
	}
	if err != nil {
		return nil, fmt.Errorf("connecting to %s: %w", addr, err)
	}

	if err := c.Login(account.Email, account.Password).Wait(); err != nil {
		c.Close()
		return nil, fmt.Errorf("login failed: %w", err)
	}
	return &Client{c: c}, nil
}

// Close closes the IMAP connection.
func (cl *Client) Close() error {
	return cl.c.Logout().Wait()
}

// FetchInbox returns the latest `limit` messages from INBOX (headers only).
func (cl *Client) FetchInbox(limit int) ([]Message, error) {
	_, err := cl.c.Select("INBOX", nil).Wait()
	if err != nil {
		return nil, fmt.Errorf("selecting INBOX: %w", err)
	}

	// Search for all messages
	searchData, err := cl.c.Search(&imap.SearchCriteria{}, nil).Wait()
	if err != nil {
		return nil, fmt.Errorf("searching messages: %w", err)
	}

	ids := searchData.AllSeqNums()
	if len(ids) == 0 {
		return []Message{}, nil
	}

	// Take the last `limit` messages
	if limit > 0 && len(ids) > limit {
		ids = ids[len(ids)-limit:]
	}

	// Reverse so newest first
	for i, j := 0, len(ids)-1; i < j; i, j = i+1, j-1 {
		ids[i], ids[j] = ids[j], ids[i]
	}

	seqSet := imap.SeqSetNum(ids...)
	fetchOptions := &imap.FetchOptions{
		Envelope: true,
		Flags:    true,
	}

	msgs, err := cl.c.Fetch(seqSet, fetchOptions).Collect()
	if err != nil {
		return nil, fmt.Errorf("fetching messages: %w", err)
	}

	var result []Message
	for _, msg := range msgs {
		m := Message{
			ID:     msg.SeqNum,
			IsRead: containsFlag(msg.Flags, imap.FlagSeen),
		}
		if msg.Envelope != nil {
			m.Subject = decodeHeader(msg.Envelope.Subject)
			m.Date = msg.Envelope.Date
			if len(msg.Envelope.From) > 0 {
				addr := msg.Envelope.From[0]
				if addr.Name != "" {
					m.From = fmt.Sprintf("%s <%s@%s>", decodeHeader(addr.Name), addr.Mailbox, addr.Host)
				} else {
					m.From = fmt.Sprintf("%s@%s", addr.Mailbox, addr.Host)
				}
			}
		}
		result = append(result, m)
	}
	return result, nil
}

// FetchMessage returns the full message including body.
func (cl *Client) FetchMessage(id uint32) (*Message, error) {
	_, err := cl.c.Select("INBOX", nil).Wait()
	if err != nil {
		return nil, fmt.Errorf("selecting INBOX: %w", err)
	}

	seqSet := imap.SeqSetNum(id)
	fetchOptions := &imap.FetchOptions{
		Envelope:    true,
		Flags:       true,
		BodySection: []*imap.FetchItemBodySection{{}},
	}

	msgs, err := cl.c.Fetch(seqSet, fetchOptions).Collect()
	if err != nil || len(msgs) == 0 {
		return nil, fmt.Errorf("message %d not found", id)
	}

	msg := msgs[0]
	m := &Message{
		ID:     msg.SeqNum,
		IsRead: containsFlag(msg.Flags, imap.FlagSeen),
	}
	if msg.Envelope != nil {
		m.Subject = decodeHeader(msg.Envelope.Subject)
		m.Date = msg.Envelope.Date
		if len(msg.Envelope.From) > 0 {
			addr := msg.Envelope.From[0]
			if addr.Name != "" {
				m.From = fmt.Sprintf("%s <%s@%s>", decodeHeader(addr.Name), addr.Mailbox, addr.Host)
			} else {
				m.From = fmt.Sprintf("%s@%s", addr.Mailbox, addr.Host)
			}
		}
	}

	// Extract body from body sections
	for _, section := range msg.BodySection {
		if len(section.Bytes) > 0 {
			m.Body = extractPlainText(section.Bytes)
			break
		}
	}

	return m, nil
}

// decodeHeader decodes RFC 2047 encoded headers (handles EUC-KR).
func decodeHeader(s string) string {
	dec := new(mime.WordDecoder)
	dec.CharsetReader = func(cs string, r io.Reader) (io.Reader, error) {
		cs = strings.ToLower(strings.ReplaceAll(cs, "-", ""))
		switch cs {
		case "euckr", "ksc56011987":
			return transform.NewReader(r, korean.EUCKR.NewDecoder()), nil
		}
		return r, nil
	}
	decoded, err := dec.DecodeHeader(s)
	if err != nil {
		return s
	}
	return decoded
}

// extractPlainText attempts to get plain text from a raw message body.
func extractPlainText(body []byte) string {
	s := string(body)
	// Very basic: strip headers, return body
	if idx := strings.Index(s, "\r\n\r\n"); idx >= 0 {
		return strings.TrimSpace(s[idx+4:])
	}
	if idx := strings.Index(s, "\n\n"); idx >= 0 {
		return strings.TrimSpace(s[idx+2:])
	}
	return strings.TrimSpace(s)
}

func containsFlag(flags []imap.Flag, flag imap.Flag) bool {
	for _, f := range flags {
		if f == flag {
			return true
		}
	}
	return false
}

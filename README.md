# tgfwd

Telegram message forwarder.

Use this tool to forward messages from one Telegram chat to another.
All type of chats are supported: private chats, groups, channels, supergroups, etc.

## 📦 Installation

You can use the Golang binary to install **tgfwd**:

```bash
go install github.com/igolaizola/tgfwd/cmd/tgfwd@latest
```

Or you can download the binary from the [releases](https://github.com/igolaizola/tgfwd/releases)

## 🛠️ Requirements

Obtain your Telegram App ID and App Hash by following this guide https://core.telegram.org/api/obtaining_api_id

## 🕹️ Usage

### Login command

Use the `login` command to login to your Telegram account and generate a session file:

```bash
tgfwd login --hash app-hash --id app-id --session file.session --phone phone-number
```

The phone number must be in the international format (e.g. +34666666666)

### List chats command

Use the `list` command to list your Telegram chats and obtain their IDs:

```bash
tgfwd list --hash app-hash --id app-id --session file.session
```

Use this command to find out the ID of the chats from which you want to forward messages and to which you want to forward them.

### Forward messages command

Use the `run` command to forward telegram messages from one chat to another:

```bash
tgfwd run --hash app-hash --id app-id --session file.session \
  --fwd from-chat-id-1:to-chat-id-2 \
  --fwd from-chat-id-3:to-chat-id-4
```

You can use the `--fwd` flag multiple times to forward messages using different chats.

### Configuration file

Instead of passing the parameters to the command, you can use a configuration file:

```bash
tgfwd run --config tgfwd.conf
```

An example configuration file `tgfwd.conf`:

```
debug true
hash app-hash
id app-id
session tgfwd.session
fwd 10000001:10000002
fwd 10000003:10000004
```

## 💖 Support

If you have found my code helpful, please give the repository a star ⭐

Additionally, if you would like to support my late-night coding efforts and the coffee that keeps me going, I would greatly appreciate a donation.

You can invite me for a coffee at ko-fi (0% fees):

[![ko-fi](https://ko-fi.com/img/githubbutton_sm.svg)](https://ko-fi.com/igolaizola)

Or at buymeacoffee:

[![buymeacoffee](https://user-images.githubusercontent.com/11333576/223217083-123c2c53-6ab8-4ea8-a2c8-c6cb5d08e8d2.png)](https://buymeacoffee.com/igolaizola)

Donate to my PayPal:

[paypal.me/igolaizola](https://www.paypal.me/igolaizola)

Sponsor me on GitHub:

[github.com/sponsors/igolaizola](https://github.com/sponsors/igolaizola)

Or donate to any of my crypto addresses:

 - BTC `bc1qvuyrqwhml65adlu0j6l59mpfeez8ahdmm6t3ge`
 - ETH `0x960a7a9cdba245c106F729170693C0BaE8b2fdeD`
 - USDT (TRC20) `TD35PTZhsvWmR5gB12cVLtJwZtTv1nroDU`
 - USDC (BEP20) / BUSD (BEP20) `0x960a7a9cdba245c106F729170693C0BaE8b2fdeD`
 - Monero `41yc4R9d9iZMePe47VbfameDWASYrVcjoZJhJHFaK7DM3F2F41HmcygCrnLptS4hkiJARCwQcWbkW9k1z1xQtGSCAu3A7V4`

Thanks for your support!

## 📚 Resources

Some of the resources I used to create this project:

 - [gotd/td](https://github.com/gotd/td)

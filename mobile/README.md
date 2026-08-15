# ezbookkeeping mobile

Android capture app for your ezbookkeeping server. Works with no signal: enter
transactions and photograph receipts while you are out, press **Upload** once
when you get home.

## What it does

1. **Connect** — point it at your server once. It swaps your password for a
   long-lived API token, so you never log in again.
2. **Capture** — add transactions and snap receipts offline. Everything lands in
   a local SQLite queue.
3. **Upload** — one button, and it returns as soon as the data is sent. All
   queued transactions go in a single batch; receipt photos are handed to the
   server's recognition queue. **The app never waits for the AI.**
4. **Review** — the server reads the receipts in the background. Press Upload
   again later and any finished ones appear here, ready for you to confirm the
   amount and category.

## Running it during development

```bash
npm install
npx expo start
```

Press `a` to open on a connected Android device or emulator. The camera works in
Expo Go, so no native build is needed just to try the screens.

## Getting a real APK on your phone

```bash
npm install -g eas-cli
eas login
eas build --platform android --profile preview
```

The `preview` profile produces an installable APK rather than a Play Store
bundle. EAS emails you a download link; open it on the phone and install.

You do **not** need Android Studio or a Mac — the build runs on Expo's servers.

## Server requirements

- `enable_api_token = true`, otherwise the app falls back to a short-lived
  session token and you will have to reconnect periodically.
- For receipt recognition: `transaction_from_ai_image_recognition = true` and a
  configured `receipt_image_recognition` LLM provider. Without it the upload
  still works, but photos come back with a "could not be read" note and you
  enter them by hand. The same setting also enables the server-side worker that
  drains the recognition queue, which runs every 15 seconds.
- Plain `http://` on a LAN is supported. If your server is on `https://` with a
  self-signed certificate, Android will reject it — use a real certificate or
  plain HTTP.

## Not supported yet

Two-factor login, transfers between accounts, currencies that do not use two
decimal places, and a home-screen widget.

# video-upload-tdbot
Upload video with cover to Telegram

## Prerequisite
- Get `API_ID` and `API_HASH` from https://core.telegram.org/api/obtaining_api_id
- Get `BOT_TOKEN` from [@BotFather](https://t.me/BotFather)
- Copy `production.env.example` to `production.env.example` and fill in all the variables
- Invite your bot into channel/group you configured just now

## Usage
- Put video file (mp4/mkv) and image cover (jpeg/jpg/png) into `tmp` folder
- Rename image cover filename to match video filename (example below)
```
my_video.mp4
my_video.png
```
- Run `docker-compose up` in current directory, after building complete, it will upload video to channel/group
- Run `docker-compose up --force-recreate --build` if you want to update image/container

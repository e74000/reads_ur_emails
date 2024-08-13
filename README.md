# \*reads_ur_emails\*

i realised a while ago that i am basically incapable of checking my emails. so instead of investing the (like a min per day max) effort required to actually look at my emails, i thought *"why don't i spend like 17hrs writing an automatic email summarisation bot?"*.[^1]

anyway...

## features
- **daily summaries:** get a summary of your emails at a specified time each day.
- **weekly summaries:** receive a comprehensive summary of your week’s emails on a specific day
- **discord integration:** summaries are sent directly to your chosen discord channels.[^2]

## setup instructions

### prerequisites
- [go](https://golang.org/doc/install) installed on your machine.
- a google cloud project with gmail api enabled.
- a discord bot with the appropriate permissions to post in the channels you designate.

### step 1: clone the repository
```sh
git clone https://github.com/yourusername/reads_ur_emails.git
cd reads_ur_emails
```

### step 2: set up google api credentials

1. **create a google cloud project:**
   - go to the [google cloud console](https://console.cloud.google.com/).
   - create a new project.

2. **enable the gmail api:**
   - in the google cloud console, navigate to "apis & services" > "library".
   - search for "gmail api" and enable it for your project.

3. **create oauth 2.0 credentials:**
   - go to "apis & services" > "credentials".
   - click "create credentials" and select "oauth 2.0 client ids".
   - download the credentials file as `credentials.json` and place it in the root directory of the project.

### step 3: configure the application

create or edit the `config.json` file in the root directory of the project with the following structure:

```json
{
  "daily_summary_time": "04:00",
  "weekly_summary_day": "monday",
  "weekly_summary_time": "05:00",
  "open_ai_key": "your_openai_api_key",
  "discord_token": "your_discord_bot_token",
  "daily_summary_channel_id": "discord_channel_id_for_daily_summary",
  "weekly_summary_channel_id": "discord_channel_id_for_weekly_summary"
}
```

- **`daily_summary_time`**: time in 24-hour format when the daily summary should be sent.
- **`weekly_summary_day`**: day of the week for the weekly summary.
- **`weekly_summary_time`**: time in 24-hour format when the weekly summary should be sent.
- **`open_ai_key`**: your openai api key.
- **`discord_token`**: your discord bot token.
- **`daily_summary_channel_id`**: the id of the discord channel where daily summaries will be posted.
- **`weekly_summary_channel_id`**: the id of the discord channel where weekly summaries will be posted.

### step 4: run the application

to run the application, execute:

```sh
go run .
```

the application will start and begin processing emails according to the schedule defined in your `config.json`.

## contributing

if you’d like to contribute to this project, add typos or improve your github contribution chart, please fork the repository and submit a pull request. contributions are welcome!

## license

this project is licensed under the MIT license - see the [license](LICENSE) file for details.

## unrelated

the task scheduler library i wrote for this is pretty neat i think, so i might break it out into its own library. dm me if i haven't already done this and you want to use it.

[^1]: [related](https://xkcd.com/974/)
[^2]: originally, i planned to make the bot send me a summary email, but i couldn't be bothered to get the gmail send email api working, and also, i wouldn't read those either.

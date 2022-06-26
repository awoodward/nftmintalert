# NFT Mint Alert
AWS Lambda function that sends Twitter and Discord alerts when a new NFT mint has been detected. NFT Mint Alert listens for event activity on the Ethereum network and analyses the activity to determine if an NFT is minting. It then uses the Opensea API to read details about the NFT project.

Project also includes a Go implementation of part of the Opensea API.

Detailed information for NFT projects is obtained from OpenSea using the OpenSea developer API. You'll need an [API Key](https://docs.opensea.io/reference/request-an-api-key) to access this API.
[OpenSea Developer API](https://docs.opensea.io/reference/api-overview)

NFT Mint Alert can post to a Discord Channel if an ID and Token are setup:
[Discord Webhook Setup](https://support.discord.com/hc/en-us/articles/228383668-Intro-to-Webhooks)

NFT Mint Alert can also post to a Twitter feed if an application key and token are setup:
[Twitter API Setup](https://developer.twitter.com/en/docs/twitter-api/getting-started/getting-access-to-the-twitter-api)

The following environment variables are set to configure the Lambda:

| Environment Variable | Description |
| :--- | :--- |
| DISCORD_WEBHOOK_ID | ID for posting to Discord Webhook |
| DISCORD_WEBHOOK_TOKEN | Secure token for posting to Discord Webhook |
| ETH_NETWORK_URL | URL for the Ethereum archive. Can be Alchemy, Infura, etc. |
| OPENSEA_API_KEY | OpenSea Developer API Key |
| S3_BUCKET | AWS S3 Bucket where status file is located |
| S3_FILE_KEY | File name of status file located in S3 bucket. It will be created if it does not exist. |
| TWITTER_CONSUMER_KEY | API Key for accessing Twitter API |
| TWITTER_CONSUMER_SECRET | API Secret for accessing Twitter API |
| TWITTER_TOKEN | OAuth user access token for the account where mint alerts will be posted |
| TWITTER_TOKEN_SECRET | OAuth user secret for the account where mint alerts will be posted |

You'll need to setup an AWS EventBridge trigger to run the Lambda process periodically the Cron expression ```0/6 * * * ? *``` will run the process every 6 minutes.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"
	"net/http"
	"nftmintalert/opensea"
	"os"
	"sort"
	"strconv"

	"github.com/andersfylling/snowflake"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/dghubble/oauth1"
	"github.com/g8rswimmer/go-twitter/v2"
	"github.com/joho/godotenv"

	twitterV1 "github.com/dghubble/go-twitter/twitter"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/nickname32/discordhook"
)

const newBlocks = 50
const contractAddressOpenSea string = "0x7Be8076f4EA4A4AD08075C2508e481d6C946D12b" // Opensea
const contractENS string = "0x283Af0B28c62C092C9727F1Ee09c02CA627EB7F5"            // ENS
const contractENS2 string = "0x57f1887a8BF19b14fC0dF6Fd9B2acc9Af147eA85"           // ENS
const nullAddress string = "0x0000000000000000000000000000000000000000"
const topicTransfer string = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"

type Event struct {
	Name string `json:"name"`
}

type Status struct {
	Recents []string `json:"recents"`
}

type MintStatus struct {
	Count int
	Value float64
}

type TwitterKeys struct {
	ConsumerKey    string
	ConsumerSecret string
	Token          string
	TokenSecret    string
}

type authorize struct {
	Token string
}

func (a authorize) Add(req *http.Request) {
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", a.Token))
}

func rankByWordCount(wordFrequencies map[string]int) PairList {
	pl := make(PairList, len(wordFrequencies))
	i := 0
	for k, v := range wordFrequencies {
		pl[i] = Pair{k, v}
		i++
	}
	sort.Sort(sort.Reverse(pl))
	return pl
}

type Pair struct {
	Key   string
	Value int
}

type PairList []Pair

func (p PairList) Len() int           { return len(p) }
func (p PairList) Less(i, j int) bool { return p[i].Value < p[j].Value }
func (p PairList) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func SetStatus(sess *session.Session, status Status, s3bucket string, s3key string) {
	buf, err := json.Marshal(status)
	svc := s3.New(sess)
	request := &s3.PutObjectInput{
		Bucket: aws.String(s3bucket),
		Key:    aws.String(s3key),
		Body:   bytes.NewReader(buf),
	}
	_, err = svc.PutObject(request)
	if err != nil {
		log.Print(err)
	}
}

func GetStatus(sess *session.Session, s3bucket string, s3key string) Status {
	// Read the current state/status from S3
	var status Status
	svc := s3.New(sess)
	requestInput := &s3.GetObjectInput{
		Bucket: aws.String(s3bucket),
		Key:    aws.String(s3key),
	}

	result, err := svc.GetObject(requestInput)
	if err != nil {
		log.Print(err)
		return status
	}

	defer result.Body.Close()
	body, err := ioutil.ReadAll(result.Body)
	if err != nil {
		log.Print(err)
	}

	bodyString := fmt.Sprintf("%s", body)
	err = json.Unmarshal([]byte(bodyString), &status)

	return status
}

func callOut(collection *opensea.OpenSeaCollection, address string, count int) bool {
	// Check Opensea details to see if it meets the criteria to tweet it.
	switch address {
	case contractENS2: //ens domains
		return false
	}
	log.Printf("Checking for callout %v (count %v)\n", address, count)
	if len(collection.ExternalLink) == 0 && len(collection.Collection.TwitterUsername) == 0 {
		return false
	}
	fmt.Println("Call out")
	return true
}

func sendDiscordWebhook(collection *opensea.OpenSeaCollection, count int, webhookId string, webhookToken string) {
	if webhookId == "" || webhookToken == "" {
		log.Println("Discord webhook Id and/or webhook token not configured.")
		return
	}
	keyInt, err := strconv.ParseInt(webhookId, 10, 64)
	if err != nil {
		log.Printf("Invalid webhook ID: %v\n", err)
		return
	}
	wa, err := discordhook.NewWebhookAPI(snowflake.Snowflake(keyInt), webhookToken, true, nil)
	if err != nil {
		log.Printf("Discord webhook error: %v\n", err)
		return
	}

	wh, err := wa.Get(nil)
	if err != nil {
		log.Printf("Discord webhook error: %v\n", err)
		return
	}
	_ = wh

	//log.Printf("Discord webhook name: %v\n", wh.Name)
	alert := fmt.Sprintf("Mint Alert!\n\n**[%v](%v)**\n\n**%v minted** in **%v minutes**\n", collection.Name, collection.Collection.ExternalURL, count, 10)

	msg, err := wa.Execute(nil, &discordhook.WebhookExecuteParams{Content: alert,
		Embeds: []*discordhook.Embed{
			{
				Image: &discordhook.EmbedImage{URL: collection.ImageURL},
			},
		},
	}, nil, "")

	if err != nil {
		panic(err)
	}

	log.Printf("Discord message sent. Message ID: %v\n", msg.ID)
}

func sendTweet(collection *opensea.OpenSeaCollection, count int, twitKey TwitterKeys) {
	if twitKey.ConsumerKey == "" {
		log.Printf("Twitter Consumer Key environment variable (TWITTER_CONSUMER_KEY) is not set.\n")
		return
	}
	if twitKey.ConsumerSecret == "" {
		log.Printf("Twitter Consumer Secret environment variable (TWITTER_CONSUMER_SECRET) is not set.\n")
		return
	}
	if twitKey.Token == "" {
		log.Printf("Twitter Token environment variable (TWITTER_TOKEN) is not set.\n")
		return
	}
	if twitKey.TokenSecret == "" {
		log.Printf("Twitter Token Secret environment variable (TWITTER_TOKEN_SECRET) is not set.\n")
		return
	}
	config := oauth1.NewConfig(twitKey.ConsumerKey, twitKey.ConsumerSecret)
	token := oauth1.NewToken(twitKey.Token, twitKey.TokenSecret)
	httpClient := config.Client(oauth1.NoContext, token)
	// Twitter client
	client := twitterV1.NewClient(httpClient)

	replyTo := ""
	if collection.Collection.TwitterUsername != "" {
		// Twitter complaint 07-16-2022 - automated @mentions
		//replyTo = "@" + collection.Collection.TwitterUsername
	}
	link := fmt.Sprintf("https://opensea.io/collection/%v", collection.Collection.Slug)
	//link := collection.ExternalLink
	status := fmt.Sprintf("NFTs Mint Alert: %v sold in 10 minutes.\n %v \nHead on over and have a look\n %v \n\n #nft #nfts #nftcollection #nftcollectibles #nftminting #niftyscoops #NFTsales", count, replyTo, link)
	sup := twitterV1.StatusUpdateParams{
		Status: status,
	}
	tweet, resp, err := client.Statuses.Update(status, &sup)
	if err != nil {
		log.Printf("Error sending tweet: (%v) %v\n", resp.StatusCode, err)
		return
	}
	log.Printf("Tweet sent. Tweet ID: %v\n", tweet.ID)
}

func sendTweetV2(collection *opensea.OpenSeaCollection, count int, twitKey TwitterKeys) {
	if twitKey.ConsumerKey == "" {
		log.Printf("Twitter Consumer Key environment variable (TWITTER_CONSUMER_KEY) is not set.\n")
		return
	}
	if twitKey.ConsumerSecret == "" {
		log.Printf("Twitter Consumer Secret environment variable (TWITTER_CONSUMER_SECRET) is not set.\n")
		return
	}
	if twitKey.Token == "" {
		log.Printf("Twitter Token environment variable (TWITTER_TOKEN) is not set.\n")
		return
	}
	if twitKey.TokenSecret == "" {
		log.Printf("Twitter Token Secret environment variable (TWITTER_TOKEN_SECRET) is not set.\n")
		return
	}
	config := oauth1.NewConfig(twitKey.ConsumerKey, twitKey.ConsumerSecret)
	token := oauth1.NewToken(twitKey.Token, twitKey.TokenSecret)
	httpClient := config.Client(oauth1.NoContext, token)

	link := fmt.Sprintf("https://opensea.io/collection/%v", collection.Collection.Slug)
	status := fmt.Sprintf("NFTs Mint Alert: %v sold in 10 minutes. \nHead on over and have a look\n %v \n\n #nft #nfts #nftcollection #nftcollectibles #nftminting #niftyscoops #NFTsales", count, link)

	client := &twitter.Client{
		Authorizer: authorize{
			Token: "",
		},
		Client: httpClient,
		Host:   "https://api.twitter.com",
	}

	req := twitter.CreateTweetRequest{
		Text: status,
	}
	fmt.Println("Callout to create tweet callout")

	tweetResponse, err := client.CreateTweet(context.Background(), req)
	if err != nil {
		log.Printf("Error sending tweet: %v\n", err)
	}

	enc, err := json.MarshalIndent(tweetResponse, "", "    ")
	if err != nil {
		log.Printf("Error unmarshaling tweet: %v\n", err)
	}
	fmt.Println(string(enc))
}

func processLogs(event Event) {
	networkUrl := os.Getenv("ETH_NETWORK_URL")
	if networkUrl == "" {
		log.Printf("Ethereum network URL environment variable (ETH_NETWORK_URL) is not set.\n")
		return
	}
	s3bucket := os.Getenv("S3_BUCKET")
	if s3bucket == "" {
		log.Printf("S3 Bucket environment variable (S3_BUCKET) is not set.\n")
		return
	}
	s3key := os.Getenv("S3_FILE_KEY")
	if s3key == "" {
		log.Printf("S3 Key environment variable (S3_FILE_KEY) is not set.\n")
		return
	}
	openseaKey := os.Getenv("OPENSEA_API_KEY")
	if s3key == "" {
		log.Printf("Opensea Key environment variable (OPENSEA_API_KEY) is not set.\n")
		return
	}

	transHashList := make(map[string]string)
	var transList []types.Log
	addressList := make(map[string]int)

	sess, err := session.NewSession(&aws.Config{
		Region: aws.String("us-east-1"),
	})
	if err != nil {
		log.Printf("Unable to create a new session %v\n", err)
		return
	}
	status := GetStatus(sess, s3bucket, s3key)

	client, err := ethclient.Dial(networkUrl)
	if err != nil {
		log.Println(err)
		return
	}

	header, err := client.HeaderByNumber(context.Background(), nil) // Get the most recent block
	if err != nil {
		log.Fatal(err)
	}
	_ = header
	toBlock := header.Number // current block
	fromBlock := big.NewInt(0).Sub(toBlock, big.NewInt(newBlocks))
	log.Printf("Start block: %v   End block: %v", fromBlock.String(), toBlock.String())

	// Query logs for transfer events
	query := ethereum.FilterQuery{
		FromBlock: fromBlock,
		ToBlock:   toBlock,
		Topics:    [][]common.Hash{{common.HexToHash(topicTransfer)}},
	}
	log.Println("Querying...")

	logs, err := client.FilterLogs(context.Background(), query)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Log entries to process: %v\n", len(logs))
	// build a list of unique transactions
	for _, vLog := range logs {
		identifier := vLog.TxHash.Hex()
		_, ok := transHashList[identifier]
		if !ok {
			transHashList[identifier] = identifier
			transList = append(transList, vLog)
		}
	}
	log.Printf("Unique transactions to process: %v\n", len(transHashList))
	// Process the logs
	lastBlock := types.Block{}
	for count, txLog := range transList {
		var topicHash common.Hash
		topicHash.SetBytes(txLog.Topics[0][:])
		topic := topicHash.Hex()
		address := txLog.Address.Hex()
		_ = lastBlock
		_ = count
		/*
			if count == 0 || txLog.BlockNumber != lastBlock.NumberU64() {
				blk, err := client.BlockByHash(context.Background(), txLog.BlockHash)
				if err != nil {
					log.Printf("Error reading block (%v): %v\n", txLog.BlockNumber, err)
				}
				lastBlock = *blk
				//fmt.Printf("Block: %v(%v) [%v] %v\n", txLog.BlockNumber, lastBlock.NumberU64(), count, txLog.BlockHash.String())
			}
		*/
		if topic != topicTransfer {
			// skip everything not a transfer
			continue
		}
		if len(txLog.Topics) < 4 {
			// ERC20 transfers have 3 topics. Skip
			continue
		}
		if address == contractAddressOpenSea || address == contractENS || address == contractENS2 {
			// skip opensea and ENS transfers.
			continue
		}

		// Count all of the transfers for an address
		fromAddr := common.BytesToAddress(txLog.Topics[1][:]).Hex()
		toAddr := common.BytesToAddress(txLog.Topics[2][:]).Hex()
		_ = toAddr
		if fromAddr == nullAddress {
			// count the mint transactions
			identifier := txLog.TxHash.Hex()
			_ = identifier
			//fmt.Printf("tx: %v cont: %v\n", identifier, address)
			count, _ := addressList[address]
			count++
			addressList[address] = count
		}

	}

	// order from most to least mint transactions
	mintlist := rankByWordCount(addressList)

	osclient := &opensea.Client{
		Client:     http.DefaultClient,
		Host:       "https://api.opensea.io",
		Authorizer: openseaKey,
	}
	discordWebhookId := os.Getenv("DISCORD_WEBHOOK_ID")
	discordWebhookToken := os.Getenv("DISCORD_WEBHOOK_TOKEN")
	twitKey := TwitterKeys{
		ConsumerKey:    os.Getenv("TWITTER_CONSUMER_KEY"),
		ConsumerSecret: os.Getenv("TWITTER_CONSUMER_SECRET"),
		Token:          os.Getenv("TWITTER_TOKEN"),
		TokenSecret:    os.Getenv("TWITTER_TOKEN_SECRET"),
	}
	for index, mint := range mintlist {
		//fmt.Printf("Key: %v val: %v\n", mint.Key, mint.Value)
		if mint.Value > 100 {
			// more than 100 mints
			// Check to see if we've already posted about this nft
			found := false
			for _, recent := range status.Recents {
				if recent == "" {
					continue
				}
				if mint.Key == recent {
					// We've already posted this NFT project
					found = true
					break
				}
			}
			if found {
				continue
			}
			collection, err := osclient.AssetContract(context.Background(), mint.Key)
			if err != nil {
				log.Printf("Opensea API error on contract %v: %v\n", mint.Key, err)
				return
			}
			result := callOut(collection, mint.Key, mint.Value)
			if result {
				log.Printf("Sending tweet. Contract: %v Slug: %v TwitterId: %v\n", mint.Key, collection.Collection.Slug, collection.Collection.TwitterUsername)
				//sendTweet(collection, mint.Value, twitKey)
				sendTweetV2(collection, mint.Value, twitKey)
				sendDiscordWebhook(collection, mint.Value, discordWebhookId, discordWebhookToken)
				// Add to list of NFT projects we've posted
				status.Recents = append(status.Recents, mint.Key)
			}
			_ = err
		}
		_ = index
	}

	if len(status.Recents) > 200 {
		// trim the oldest from the list
		status.Recents = status.Recents[2:]
	}
	SetStatus(sess, status, s3bucket, s3key)
	log.Println("End")

	//fmt.Println(addressList)
}

func HandleRequest(ctx context.Context, event Event) {
	processLogs(event)
	return
}

func init() {
	godotenv.Load()
}

func main() {
	lambda.Start(HandleRequest)

	//processLogs(Event{})
}

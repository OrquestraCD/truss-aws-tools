package main

import (
	"github.com/trussworks/truss-aws-tools/internal/aws/session"
	"github.com/trussworks/truss-aws-tools/pkg/amiclean"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	flag "github.com/jessevdk/go-flags"
	"go.uber.org/zap"

	"log"
	"time"
)

// The Options struct describes the command line options available.
type Options struct {
	Delete        bool   `short:"D" long:"delete" description:"Actually purge AMIs (runs in dryrun mode by default)."`
	NamePrefix    string `long:"prefix" description:"Name prefix to filter on (not affected by --invert)."`
	RetentionDays int    `long:"days" default:"30" description:"Age of AMI in days before it is a candidate for removal."`
	TagKey        string `long:"tag-key" description:"Key of tag to operate on. If you specify a Key, you must also specify a Value."`
	TagValue      string `long:"tag-value" description:"Value of tag to operate on. If you specify a Value, you must also specify a Key."`
	Invert        bool   `short:"i" long:"invert" description:"Operate in inverted mode -- only purge AMIs that do NOT match the Tag provided."`
	Profile       string `short:"p" long:"profile" env:"PROFILE" required:"false" description:"The AWS profile to use."`
	Region        string `short:"r" long:"region" env:"REGION" required:"false" description:"The AWS region to use."`
	Lambda        bool   `long:"lambda" required:"false" env:"LAMBDA" description:"Run as an AWS Lambda function."`
}

var options Options
var logger *zap.Logger

// This function is for establishing our session with AWS.
func makeEC2Client(region, profile string) *ec2.EC2 {
	sess := session.MustMakeSession(region, profile)
	ec2Client := ec2.New(sess)
	return ec2Client
}

func cleanImages() {
	now := time.Now().UTC()
	// We need to check to make sure that if we have a Tag Key, we also have
	// a Tag Value.
	if (options.TagKey == "") != (options.TagValue == "") {
		logger.Fatal("must specify both a tag Key and tag Value")
	}

	a := amiclean.AMIClean{
		NamePrefix:     options.NamePrefix,
		Tag:            &ec2.Tag{Key: aws.String(options.TagKey), Value: aws.String(options.TagValue)},
		Delete:         options.Delete,
		Invert:         options.Invert,
		ExpirationDate: now.AddDate(0, 0, -int(options.RetentionDays)),
		Logger:         logger,
		EC2Client:      makeEC2Client(options.Region, options.Profile),
	}

	availableImages, err := a.GetImages()
	if err != nil {
		logger.Fatal("unable to get list of available images",
			zap.Error(err),
		)
	}

	purgeList := a.FindImagesToPurge(availableImages)

	err = a.PurgeImages(purgeList)
	if err != nil {
		logger.Fatal("unable to complete image purge",
			zap.Error(err),
		)
	}

}

func lambdaHandler() {
	lambda.Start(cleanImages)
}

func main() {
	// First, parse out our command line options:
	parser := flag.NewParser(&options, flag.Default)
	_, err := parser.Parse()
	if err != nil {
		log.Fatal(err)
	}

	// Initialize the zap logger:
	logger, err = zap.NewProduction()
	if err != nil {
		log.Fatalf("can't initialize zap logger: %v", err)
	}

	// We need to check to see if we were called as a Lambda function.
	if options.Lambda {
		logger.Info("Running Lambda handler.")
		lambdaHandler()
	} else {
		cleanImages()
	}

}

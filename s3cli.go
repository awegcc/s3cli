package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/endpoints"
	"github.com/aws/aws-sdk-go-v2/aws/external"

	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/spf13/cobra"
)

// version to record s3cli version
var version = "1.2.3"

// endpoint ENV Var
var endpointEnvVar = "S3_ENDPOINT"

// S3Cli represent a S3Cli Client
type S3Cli struct {
	profile  string // profile in credentials file
	endpoint string // Server endpoine(URL)
	region   string
	verbose  bool
	debug    bool
}

func (sc *S3Cli) loadS3Cfg() (*aws.Config, error) {
	cfg, err := external.LoadDefaultAWSConfig(external.WithSharedConfigProfile(sc.profile))
	if err != nil {
		return nil, fmt.Errorf("failed to load config, %v", err)
	}
	cfg.Region = sc.region
	//cfg.EndpointResolver = aws.ResolveWithEndpoint{
	//	URL: sc.endpoint,
	//}
	defaultResolver := endpoints.NewDefaultResolver()
	myCustomResolver := func(service, region string) (aws.Endpoint, error) {
		if service == s3.EndpointsID {
			return aws.Endpoint{
				URL: sc.endpoint,
				//SigningRegion: "custom-signing-region",
				SigningNameDerived: true,
			}, nil
		}
		return defaultResolver.ResolveEndpoint(service, region)
	}
	cfg.EndpointResolver = aws.EndpointResolverFunc(myCustomResolver)
	return &cfg, nil
}

// newS3Client allocate a s3.Client
func (sc *S3Cli) newS3Client() (*s3.Client, error) {
	cfg, err := sc.loadS3Cfg()
	if err != nil {
		return nil, err
	}
	if sc.debug {
		fmt.Println(cfg)
	}
	client := s3.New(*cfg)
	if sc.endpoint == "" {
		sc.endpoint = os.Getenv(endpointEnvVar)
	}
	if sc.endpoint != "" {
		client.ForcePathStyle = true
	}
	return client, nil
}

func splitBucketObject(bucketObject string) (bucket, object string) {
	bo := strings.SplitN(bucketObject, "/", 2)
	if len(bo) == 2 {
		return bo[0], bo[1]
	}
	return bucketObject, ""
}

// listAllObjects list all Objects in spcified bucket
func (sc *S3Cli) listAllObjects(bucket, prefix, delimiter string, index bool) error {
	client, err := sc.newS3Client()
	if err != nil {
		return fmt.Errorf("init s3 Client failed: %v", err)
	}
	var i int64
	req := client.ListObjectsRequest(&s3.ListObjectsInput{
		Bucket:    aws.String(bucket),
		Prefix:    aws.String(prefix),
		Delimiter: aws.String(delimiter),
	})
	p := s3.NewListObjectsPaginator(req)
	for p.Next(context.TODO()) {
		page := p.CurrentPage()
		if sc.verbose {
			fmt.Println(page)
			continue
		}
		for _, obj := range page.Contents {
			if index {
				fmt.Printf("%d\t%s\n", i, *obj.Key)
				i++
			} else {
				fmt.Println(*obj.Key)
			}
		}
	}
	if err := p.Err(); err != nil {
		return fmt.Errorf("list all objects failed: %v", err)
	}
	return nil
}

// listObjects list Objects in spcified bucket
func (sc *S3Cli) listObjects(bucket, prefix, delimiter, marker string, maxkeys int64, index bool) error {
	client, err := sc.newS3Client()
	if err != nil {
		return fmt.Errorf("init s3 Client failed: %v", err)
	}
	req := client.ListObjectsRequest(&s3.ListObjectsInput{
		Bucket:    aws.String(bucket),
		Prefix:    aws.String(prefix),
		Marker:    aws.String(marker),
		Delimiter: aws.String(delimiter),
		MaxKeys:   aws.Int64(maxkeys),
	})
	resp, err := req.Send(context.Background())
	if err != nil {
		return fmt.Errorf("list objects failed: %v", err)
	}
	if sc.verbose {
		fmt.Println(resp)
		return nil
	}
	for _, p := range resp.CommonPrefixes {
		fmt.Println(*p.Prefix)
	}
	for i, obj := range resp.Contents {
		if index {
			fmt.Printf("%d\t%s\n", i, *obj.Key)
		} else {
			fmt.Println(*obj.Key)
		}
	}
	return nil
}

// renameObjects rename Object(s)
func (sc *S3Cli) renameObjects(bucket, prefix, delimiter, marker string) error {
	// TODO: Copy and Delete Object
	return fmt.Errorf("not impl")

}

// copyObjects copy Object to destBucket/key
func (sc *S3Cli) copyObject(source, bucket, key string) error {
	client, err := sc.newS3Client()
	if err != nil {
		return fmt.Errorf("init s3 Client failed: %v", err)
	}
	req := client.CopyObjectRequest(&s3.CopyObjectInput{
		CopySource: aws.String(source),
		Bucket:     aws.String(bucket),
		Key:        aws.String(key),
	})
	resp, err := req.Send(context.Background())
	if err != nil {
		return fmt.Errorf("copy object failed: %v", err)
	}
	if sc.verbose {
		fmt.Println(resp)
		return nil
	}
	return nil
}

// getObject download a Object from bucket
func (sc *S3Cli) getObject(bucket, key, oRange, filename string) error {
	client, err := sc.newS3Client()
	if err != nil {
		return fmt.Errorf("init s3 Client failed: %v", err)
	}
	// Create a file to write the S3 Object contents to.
	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file %q, %v", filename, err)
	}
	defer f.Close()
	var objRange *string
	if oRange != "" {
		objRange = aws.String(fmt.Sprintf("bytes=%s", oRange))
	}
	req := client.GetObjectRequest(&s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Range:  objRange,
	})
	resp, err := req.Send(context.Background())
	if err != nil {
		return fmt.Errorf("get object failed: %v", err)
	}
	_, err = io.Copy(f, resp.Body)
	return err
}

// catObject print Object contents
func (sc *S3Cli) catObject(bucket, key, oRange string) error {
	client, err := sc.newS3Client()
	if err != nil {
		return fmt.Errorf("init s3 Client failed: %v", err)
	}
	var objRange *string
	if oRange != "" {
		objRange = aws.String(fmt.Sprintf("bytes=%s", oRange))
	}
	req := client.GetObjectRequest(&s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Range:  objRange,
	})
	resp, err := req.Send(context.Background())
	if err != nil {
		return fmt.Errorf("get object failed: %v", err)
	}
	_, err = io.Copy(os.Stdout, resp.Body)
	return err
}

// putObject upload a Object
func (sc *S3Cli) putObject(bucket, key, filename string) error {
	client, err := sc.newS3Client()
	if err != nil {
		return fmt.Errorf("init s3 Client failed: %v", err)
	}
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	req := client.PutObjectRequest(&s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   f,
	})
	_, err = req.Send(context.Background())
	return err
}

func (sc *S3Cli) headObject(bucket, key string, mtime, mtimestamp bool) error {
	client, err := sc.newS3Client()
	if err != nil {
		return fmt.Errorf("init s3 Client failed: %v", err)
	}
	req := client.HeadObjectRequest(&s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	resp, err := req.Send(context.Background())
	if err != nil {
		return err
	}
	if resp == nil {
		return nil
	}
	if sc.verbose {
		fmt.Println(resp.HeadObjectOutput)
	} else if mtime {
		fmt.Println(resp.HeadObjectOutput.LastModified)
	} else if mtimestamp {
		fmt.Println(resp.HeadObjectOutput.LastModified.Unix())
	} else {
		fmt.Printf("%d\t%s\n", *resp.HeadObjectOutput.ContentLength, resp.HeadObjectOutput.LastModified)
	}
	return nil
}

func (sc *S3Cli) deleteObjects(bucket, prefix string, wait time.Duration) error {
	client, err := sc.newS3Client()
	if err != nil {
		return fmt.Errorf("init s3 Client failed: %v", err)
	}
	var objNum int64
	loi := &s3.ListObjectsInput{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	}
	for {
		req := client.ListObjectsRequest(loi)
		resp, err := req.Send(context.Background())
		if err != nil {
			return fmt.Errorf("list object failed: %v", err)
		}
		objectNum := len(resp.Contents)
		if objectNum == 0 {
			break
		}
		if sc.verbose {
			fmt.Printf("Got %d Objects, ", objectNum)
		}
		objects := make([]s3.ObjectIdentifier, 0, 1000)
		for _, obj := range resp.Contents {
			objects = append(objects, s3.ObjectIdentifier{Key: obj.Key})
		}
		doi := &s3.DeleteObjectsInput{
			Bucket: aws.String(bucket),
			Delete: &s3.Delete{Quiet: aws.Bool(true),
				Objects: objects},
		}
		deleteReq := client.DeleteObjectsRequest(doi)
		if _, e := deleteReq.Send(context.Background()); err != nil {
			fmt.Printf("delete Objects failed: %s", e)
		} else {
			objNum = objNum + int64(objectNum)
		}
		if sc.verbose {
			fmt.Printf("%d Objects deleted\n", objNum)
		}
		if wait > 0 {
			time.Sleep(wait)
		}
		if resp.NextMarker != nil {
			loi.Marker = resp.NextMarker
		} else if resp.IsTruncated != nil && *resp.IsTruncated {
			loi.Marker = resp.Contents[objectNum-1].Key
		} else {
			break
		}
	}
	return nil
}

// deleteBucketAndObjects force delete a Bucket
func (sc *S3Cli) deleteBucketAndObjects(bucket string, force bool) error {
	if force {
		if err := sc.deleteObjects(bucket, "", 0); err != nil {
			return err
		}
	}
	return sc.deleteBucket(bucket)
}

func (sc *S3Cli) deleteObject(bucket, key string) error {
	client, err := sc.newS3Client()
	if err != nil {
		return fmt.Errorf("init s3 Client failed: %v", err)
	}
	req := client.DeleteObjectRequest(&s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	_, err = req.Send(context.Background())
	return err
}

func (sc *S3Cli) policyBucket(bucket, key string) error {
	client, err := sc.newS3Client()
	if err != nil {
		return fmt.Errorf("init s3 Client failed: %v", err)
	}
	req := client.GetBucketPolicyRequest(&s3.GetBucketPolicyInput{
		Bucket: aws.String(bucket),
	})
	resp, err := req.Send(context.Background())
	if err != nil {
		return err
	}
	fmt.Println(*resp.GetBucketPolicyOutput.Policy)
	return nil
}

// mpuObject Multi-Part-Upload a Object
// TODO: impl
func (sc *S3Cli) mpuObject(bucket, key, filename string) error {
	client, err := sc.newS3Client()
	if err != nil {
		return fmt.Errorf("init s3 Client failed: %v", err)
	}
	// Create a file to write the S3 Object contents to.
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	req := client.CreateMultipartUploadRequest(&s3.CreateMultipartUploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	req.SetReaderBody(f)
	_, err = req.Send(context.Background())
	return err
}

// presignGetObject presign a URL to download Object
func (sc *S3Cli) presignGetObject(bucket, key string, exp time.Duration) (string, error) {
	client, err := sc.newS3Client()
	if err != nil {
		return "", fmt.Errorf("init s3 Client failed: %v", err)
	}
	req := client.GetObjectRequest(&s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	return req.Presign(exp)
}

// presignPutObject presing a URL to uploda Object
func (sc *S3Cli) presignPutObject(bucket, key string, exp time.Duration) (string, error) {
	client, err := sc.newS3Client()
	if err != nil {
		return "", fmt.Errorf("init s3 Client failed: %v", err)
	}
	req := client.PutObjectRequest(&s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	return req.Presign(exp)
}

func (sc *S3Cli) getObjectACL(bucket, key string) error {
	client, err := sc.newS3Client()
	if err != nil {
		return fmt.Errorf("init s3 Client failed: %v", err)
	}

	req := client.GetObjectAclRequest(&s3.GetObjectAclInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	resp, err := req.Send(context.Background())
	if err != nil {
		return err
	}
	if resp != nil {
		fmt.Println(resp.GetObjectAclOutput)
	}
	return nil
}

func (sc *S3Cli) createBucket(bucket string) error {
	client, err := sc.newS3Client()
	if err != nil {
		return fmt.Errorf("init s3 Client failed: %v", err)
	}
	createBucketReq := client.CreateBucketRequest(&s3.CreateBucketInput{
		Bucket: aws.String(bucket),
		CreateBucketConfiguration: &s3.CreateBucketConfiguration{
			LocationConstraint: s3.BucketLocationConstraint(sc.region),
		},
	})
	_, err = createBucketReq.Send(context.Background())
	return err
}

func (sc *S3Cli) getBucketACL(bucket string) error {
	client, err := sc.newS3Client()
	if err != nil {
		return fmt.Errorf("init s3 Client failed: %v", err)
	}
	req := client.GetBucketAclRequest(&s3.GetBucketAclInput{
		Bucket: aws.String(bucket),
	})
	resp, err := req.Send(context.Background())
	if err != nil {
		return err
	}
	if resp != nil {
		fmt.Println(resp.GetBucketAclOutput)
	}
	return err
}

func (sc *S3Cli) headBucket(bucket string) error {
	client, err := sc.newS3Client()
	if err != nil {
		return fmt.Errorf("init s3 Client failed: %v", err)
	}
	req := client.HeadBucketRequest(&s3.HeadBucketInput{
		Bucket: aws.String(bucket),
	})
	resp, err := req.Send(context.Background())
	if err != nil {
		return err
	}
	if resp != nil {
		fmt.Println(resp.HeadBucketOutput)
	}
	return err
}

func (sc *S3Cli) deleteBucket(bucket string) error {
	client, err := sc.newS3Client()
	if err != nil {
		return fmt.Errorf("init s3 Client failed: %v", err)
	}
	req := client.DeleteBucketRequest(&s3.DeleteBucketInput{
		Bucket: aws.String(bucket),
	})
	_, err = req.Send(context.Background())
	return err
}

func (sc *S3Cli) listBuckets() error {
	client, err := sc.newS3Client()
	if err != nil {
		return fmt.Errorf("init s3 Client failed: %v", err)
	}
	req := client.ListBucketsRequest(&s3.ListBucketsInput{})
	resp, err := req.Send(context.Background())
	if err != nil {
		return err
	}
	if sc.verbose {
		fmt.Println(resp.ListBucketsOutput)
		return nil
	}
	for _, b := range resp.ListBucketsOutput.Buckets {
		fmt.Println(*b.Name)
	}
	return nil
}

func main() {
	sc := S3Cli{}
	var rootCmd = &cobra.Command{
		Use:   "s3cli",
		Short: "s3cli client tool",
		Long: `S3 commandline tool
Endpoint Envvar:
	S3_ENDPOINT=http://host:port (only read if flag -e is not set)

Credential Envvar:
	AWS_ACCESS_KEY_ID=AK      (only read if flag -p is not set)
	AWS_ACCESS_KEY=AK         (only read if AWS_ACCESS_KEY_ID is not set)
	AWS_SECRET_ACCESS_KEY=SK  (only read if flag -p is not set)
	AWS_SECRET_KEY=SK         (only read if AWS_SECRET_ACCESS_KEY is not set)`,
		Version: version,
	}
	rootCmd.PersistentFlags().BoolVarP(&sc.debug, "debug", "", false, "print debug log")
	rootCmd.PersistentFlags().BoolVarP(&sc.verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().StringVarP(&sc.endpoint, "endpoint", "e", "", "S3 endpoint(http://host:port)")
	rootCmd.PersistentFlags().StringVarP(&sc.profile, "profile", "p", "", "profile in credentials file")
	rootCmd.PersistentFlags().StringVarP(&sc.region, "region", "R", endpoints.CnNorth1RegionID, "region")

	createBucketCmd := &cobra.Command{
		Use:     "createBucket <bucket>",
		Aliases: []string{"cb", "mb"},
		Short:   "create(make) Bucket",
		Long: `create Bucket
1. createBucket alias
  s3cli cb Bucket
2. or makeBucket alias
  s3cli mb Bucket`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			sc.createBucket(args[0])
		},
	}
	rootCmd.AddCommand(createBucketCmd)

	headCmd := &cobra.Command{
		Use:     "head <bucket/key>",
		Aliases: []string{"head"},
		Short:   "head Bucket/Object",
		Long: `get Bucket/Object metadata
1. get a Bucket's Metadata
 s3cli head Bucket
2. get a Object's Metadata
 s3cli head Bucket/Key`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			bucket, key := splitBucketObject(args[0])
			if key != "" {
				mt := cmd.Flag("mtime").Changed
				mts := cmd.Flag("mtimestamp").Changed
				if err := sc.headObject(bucket, key, mt, mts); err != nil {
					fmt.Printf("head %s/%s failed: %s\n", bucket, key, err)
				}
			} else {
				if err := sc.headBucket(bucket); err != nil {
					fmt.Printf("head %s failed: %s\n", bucket, err)
				}
			}
		},
	}
	headCmd.Flags().BoolP("mtimestamp", "", false, "show Object mtimestamp")
	headCmd.Flags().BoolP("mtime", "", false, "show Object mtime")
	rootCmd.AddCommand(headCmd)

	aclCmd := &cobra.Command{
		Use:   "acl <bucket/key>",
		Short: "get Bucket/Object ACL",
		Long: `get Bucket/Object ACL
1. get a Bucket's ACL
 s3cli acl Bucket
2. get a Object's ACL
 s3cli acl Bucket/Key`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			bucket, key := splitBucketObject(args[0])
			if key != "" {
				if err := sc.getObjectACL(bucket, key); err != nil {
					fmt.Printf("get %s/%s ACL failed: %s\n", bucket, key, err)
				}
			} else {
				if err := sc.getBucketACL(bucket); err != nil {
					fmt.Printf("get %s ACL failed: %s\n", bucket, err)
				}
			}
		},
	}
	rootCmd.AddCommand(aclCmd)

	putObjectCmd := &cobra.Command{
		Use:     "upload <local-file> <bucket/key>",
		Aliases: []string{"put", "up", "u"},
		Short:   "upload Object",
		Long: `upload Object to Bucket
1. upload a file
  s3cli up /path/to/file Bucket
2. upload a file to Bucket/Key
  s3cli up /path/to/file Bucket/Key`,
		Args: cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			bucket, key := splitBucketObject(args[1])
			if key == "" {
				key = filepath.Base(args[0])
			}
			if err := sc.putObject(bucket, key, args[0]); err != nil {
				fmt.Printf("upload %s failed: %s\n", args[1], err)
			} else {
				fmt.Printf("upload %s to %s success\n", args[0], args[1])
			}
		},
	}
	rootCmd.AddCommand(putObjectCmd)

	mpuObjectCmd := &cobra.Command{
		Use:     "mpu <local-file> <bucket/key>",
		Aliases: []string{"mp", "mu"},
		Short:   "mpu Object",
		Long: `mutiPartUpload Object to Bucket
1. upload a file
  s3cli up /path/to/file Bucket
2. upload a file Bucket/Key
  s3cli up /path/to/file Bucket/Key`,
		Args: cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			bucket, key := splitBucketObject(args[1])
			if key == "" {
				key = filepath.Base(args[0])
			}
			if err := sc.mpuObject(bucket, key, args[0]); err != nil {
				fmt.Printf("mpu %s failed: %s\n", key, err)
			} else {
				fmt.Printf("mpu %s success\n", key)
			}
		},
	}
	rootCmd.AddCommand(mpuObjectCmd)

	listObjectCmd := &cobra.Command{
		Use:     "list [bucket[/prefix]]",
		Aliases: []string{"ls"},
		Short:   "list Buckets or Objects",
		Long: `list Buckets or Objects
1. list Buckets
  s3cli ls
2. list Objects
  s3cli ls Bucket
3. list Objects with prefix(2019)
  s3cli ls Bucket/2019`,
		Args: cobra.RangeArgs(0, 1),
		Run: func(cmd *cobra.Command, args []string) {
			index := cmd.Flag("index").Changed
			delimiter := cmd.Flag("delimiter").Value.String()
			if len(args) == 1 {
				bucket, prefix := splitBucketObject(args[0])
				if cmd.Flag("all").Changed {
					if err := sc.listAllObjects(bucket, prefix, delimiter, index); err != nil {
						fmt.Println(err)
					}
				} else {
					maxKeys, err := cmd.Flags().GetInt64("maxkeys")
					if err != nil {
						maxKeys = 1000
					}
					marker := cmd.Flag("marker").Value.String()
					if err := sc.listObjects(bucket, prefix, delimiter, marker, maxKeys, index); err != nil {
						fmt.Println(err)
					}
				}
			} else {
				if err := sc.listBuckets(); err != nil {
					fmt.Println(err)
				}
			}
		},
	}
	listObjectCmd.Flags().StringP("marker", "m", "", "marker")
	listObjectCmd.Flags().Int64P("maxkeys", "M", 1000, "max keys")
	listObjectCmd.Flags().StringP("delimiter", "d", "", "Object delimiter")
	listObjectCmd.Flags().BoolP("index", "i", false, "show Object index ")
	listObjectCmd.Flags().BoolP("all", "a", false, "list all Objects")
	rootCmd.AddCommand(listObjectCmd)

	getObjectCmd := &cobra.Command{
		Use:     "download <bucket/key> [destination]",
		Aliases: []string{"get", "down", "d"},
		Short:   "download Object",
		Long: `download Object from Bucket
1. download a Object to PWD
  s3cli down Bucket/Key
2. download a Object to /path/to/file
  s3cli down Bucket/Key /path/to/file`,
		Args: cobra.RangeArgs(1, 2),
		Run: func(cmd *cobra.Command, args []string) {
			bucket, key := splitBucketObject(args[0])
			destination := ""
			if len(args) == 2 {
				destination = args[1]
			} else {
				destination = filepath.Base(key)
			}
			objRange := cmd.Flag("range").Value.String()
			if err := sc.getObject(bucket, key, objRange, destination); err != nil {
				fmt.Printf("download %s to %s failed: %s\n", args[1], destination, err)
			} else {
				fmt.Printf("download %s to %s\n", args[0], destination)
			}
		},
	}
	getObjectCmd.Flags().StringP("range", "r", "", "Object range to download, 0-64 means [0, 64]")
	getObjectCmd.Flags().BoolP("overwrite", "w", false, "overwrite file if exist")
	rootCmd.AddCommand(getObjectCmd)

	catObjectCmd := &cobra.Command{
		Use:   "cat <bucket/key>",
		Short: "cat Object",
		Long: `cat Object contents
1. cat Object
  s3cli cat Bucket/Key`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			objRange := cmd.Flag("range").Value.String()
			bucket, key := splitBucketObject(args[0])
			if err := sc.catObject(bucket, key, objRange); err != nil {
				fmt.Printf("cat %s failed: %s\n", args[0], err)
			}
		},
	}
	catObjectCmd.Flags().StringP("range", "r", "", "Object range to cat, 0-64 means [0, 64]")
	rootCmd.AddCommand(catObjectCmd)

	copyObjectCmd := &cobra.Command{
		Use:     "copy <bucket/key> <bucket/key>",
		Aliases: []string{"cp"},
		Short:   "copy Object",
		Long: `copy Bucket/key to Bucket/key
1. spedify destination key
  s3cli copy Bucket1/Key1 Bucket2/Key2
2. default destionation key
  s3cli copy Bucket1/Key1 Bucket2`,
		Args: cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			bucket, key := splitBucketObject(args[1])
			if key == "" {
				_, key = splitBucketObject(args[0])
			}
			if err := sc.copyObject(args[0], bucket, key); err != nil {
				fmt.Printf("copy %s failed: %s\n", args[1], err)
			}
		},
	}
	rootCmd.AddCommand(copyObjectCmd)

	deleteObjectCmd := &cobra.Command{
		Use:     "delete <bucket/key>",
		Aliases: []string{"del", "rm"},
		Short:   "delete(remove) Object or Bucket(Bucket and Objects)",
		Long: `delete Bucket or Object(s)
1. delete Bucket and all Objects
  s3cli delete Bucket
2. delete Object
  s3cli delete Bucket/Key
3. delete all Objects with same Prefix
  s3cli delete Bucket/Prefix -x`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			wait, err := time.ParseDuration(cmd.Flag("wait").Value.String())
			if err != nil {
				fmt.Println("invalid wait: ", err)
				return
			}
			prefixMode := cmd.Flag("prefix").Changed
			force := cmd.Flag("force").Changed
			bucket, key := splitBucketObject(args[0])
			if prefixMode {
				if err := sc.deleteObjects(bucket, key, wait); err != nil {
					fmt.Println("delete Objects failed: ", err)
				}
			} else if key != "" {
				if err := sc.deleteObject(bucket, key); err != nil {
					fmt.Println("delete Object failed: ", err)
				}
			} else {
				if err := sc.deleteBucketAndObjects(bucket, force); err != nil {
					fmt.Printf("deleted Bucket %s and Objects failed: %s\n", args[0], err)
				}
			}
		},
	}
	deleteObjectCmd.Flags().BoolP("force", "", false, "delete Bucket and all Objects")
	deleteObjectCmd.Flags().DurationP("wait", "", 1*time.Second, "wait until next list")
	deleteObjectCmd.Flags().BoolP("prefix", "x", false, "delete Objects start with specified prefix")
	rootCmd.AddCommand(deleteObjectCmd)

	presignObjectCmd := &cobra.Command{
		Use:     "presign <bucket/key>",
		Aliases: []string{"ps"},
		Short:   "presign Object",
		Long: `presign Object URL
1. presign a Get URL
  s3cli presign Bucket/Key
2. presign a Put URL
  s3cli presign Bucket/Key --put`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			exp, err := time.ParseDuration(cmd.Flag("expire").Value.String())
			if err != nil {
				fmt.Println("invalid expire : ", err)
				return
			}
			bucket, key := splitBucketObject(args[0])
			var url string
			if cmd.Flag("put").Changed {
				url, err = sc.presignPutObject(bucket, key, exp)
			} else {
				url, err = sc.presignGetObject(bucket, key, exp)
			}
			if err != nil {
				fmt.Println("presign failed: ", err)
			} else {
				fmt.Println(url)
			}
		},
	}
	presignObjectCmd.Flags().DurationP("expire", "E", 12*time.Hour, "URL expire time")
	presignObjectCmd.Flags().BoolP("put", "", false, "generate a put URL")
	rootCmd.AddCommand(presignObjectCmd)

	policyCmd := &cobra.Command{
		Use:   "policy <bucket/key>",
		Short: "policy Bucket or Object",
		Long: `policy Bucket or Object(s) in Bucket
1. policy Bucket
  s3cli policy Bucket
2. policy Object
  s3cli policy Bucket/Key`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			//prefix := cmd.Flag("prefix").Changed
			bucket, key := splitBucketObject(args[0])
			if err := sc.policyBucket(bucket, key); err != nil {
				fmt.Printf("policy failed: %v\n", err)
			} else {
				fmt.Printf(" Objects success\n")
			}
		},
	}
	rootCmd.AddCommand(policyCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

# hyperdrive-monolith
Authentication for AWS Cloudfront

## Installation

```bash
aws cloudformation package \
    --profile deepimpact-dev-nv \
    --template-file template.yml \
    --s3-bucket lambdacfartifacts-artifactbucket-1lj1o7ro1f4bh \
    --s3-prefix lambda \
    --output-template-file packaged-template.yml
```

```bash
aws cloudformation deploy \
  --profile deepimpact-dev-nv \
  --template-file packaged-template.yml \
  --capabilities CAPABILITY_IAM \
  --stack-name Test3Auth \
  --parameter-override \
    UserPoolId="???" \
    ProtectedDomainName="test3-hyperdrive.first-impact.io"
```
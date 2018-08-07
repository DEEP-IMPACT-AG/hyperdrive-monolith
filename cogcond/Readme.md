```bash
aws cloudformation deploy \
  --profile deepimpact-dev \
  --template-file packaged-template.yml \
  --capabilities CAPABILITY_IAM \
  --stack-name HyperdriveCogCond
```
# BCRDF Setup Guide

## S3 Configuration

BCRDF uses AWS S3 to store backups. Here's how to configure your environment:

### 1. Create S3 Bucket

1. Sign in to [AWS Console](https://console.aws.amazon.com/)
2. Go to S3 service
3. Click "Create bucket"
4. Choose a unique name for your bucket (e.g., `bcrdf-backups`)
5. Select your preferred region
6. Configure security options according to your needs
7. Click "Create bucket"

### 2. Create IAM User

1. Go to IAM service
2. Click "Users" then "Create user"
3. Give a name to the user (e.g., `bcrdf-backup-user`)
4. Attach the following policy:

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "s3:GetObject",
                "s3:PutObject",
                "s3:DeleteObject",
                "s3:ListBucket"
            ],
            "Resource": [
                "arn:aws:s3:::bcrdf-backups",
                "arn:aws:s3:::bcrdf-backups/*"
            ]
        }
    ]
}
```

### 3. Generate Access Keys

1. Select the created user
2. Go to "Security credentials" tab
3. Click "Create access key"
4. Choose "Application running outside AWS"
5. Copy the Access Key ID and Secret Access Key

### 4. Configure BCRDF

1. Copy the configuration file:
```bash
cp configs/config-s3-complete.yaml config.yaml
```

2. Edit `config.yaml` with your parameters:
```yaml
storage:
  type: "s3"
  bucket: "bcrdf-backups"  # Your bucket name
  region: "us-east-1"      # Your region
  endpoint: "https://s3.us-east-1.amazonaws.com"
  access_key: "AKIA..."    # Your Access Key ID
  secret_key: "..."        # Your Secret Access Key

backup:
  encryption_key: "your-very-secure-encryption-key-32-chars"
  compression_level: 3
  max_workers: 10
```

### 5. Environment Variables (Alternative)

Instead of putting keys in the configuration file, you can use environment variables:

```bash
export AWS_ACCESS_KEY_ID="AKIA..."
export AWS_SECRET_ACCESS_KEY="..."
```

In this case, leave `access_key` and `secret_key` empty in `config.yaml`.

## Configuration Test

After configuring S3, test the connection:

```bash
# Build the application
make build

# Test a backup
./bcrdf backup --source ./test-data --name "test-backup"
```

## Security

### Encryption Key Generation

Generate a secure encryption key:

```bash
# Generate AES-256-GCM key
./scripts/generate-key.sh

# Or manually create a 32-byte key
openssl rand -hex 32
```

### Best Practices

1. **Use IAM roles** instead of access keys when possible
2. **Rotate access keys** regularly
3. **Use bucket policies** to restrict access
4. **Enable versioning** on your S3 bucket
5. **Set up monitoring** with CloudTrail

## WebDAV Configuration

### Nextcloud Setup

1. Create a Nextcloud account
2. Generate an app password
3. Use the WebDAV URL: `https://your-nextcloud.com/remote.php/dav/files/username/`

### Configuration Example

```yaml
storage:
  type: "webdav"
  endpoint: "https://your-nextcloud.com/remote.php/dav/files/username/"
  username: "your-username"
  password: "your-app-password"
```

## Testing

### Connectivity Test

```bash
# Test S3 configuration
./bcrdf init config.yaml --test

# Test WebDAV configuration
./bcrdf init config-webdav.yaml --test
```

### Complete Test

```bash
# Create test data
mkdir -p /tmp/test-data
echo "test content" > /tmp/test-data/test.txt

# Perform backup
./bcrdf backup -n "test-backup" -s "/tmp/test-data" --config config.yaml

# List backups
./bcrdf list --config config.yaml

# Restore backup
./bcrdf restore -b "backup-id" -d "/tmp/restore" --config config.yaml
```

## Troubleshooting

### Common Issues

#### S3 Connectivity
- Check your access keys
- Verify bucket permissions
- Ensure region is correct
- Check network connectivity

#### WebDAV Connectivity
- Verify username/password
- Check WebDAV URL format
- Ensure SSL certificate is valid
- Test with curl: `curl -u username:password -X PROPFIND https://your-webdav-url/`

#### Performance Issues
- Increase `max_workers` for better parallelism
- Adjust `buffer_size` based on your system
- Use `compression_level: 1` for faster backups
- Consider using `checksum_mode: "fast"` for large datasets 
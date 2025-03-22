#!/bin/bash
# Script to package and deploy the s3-access action

set -e

# Create a temporary directory
TEMP_DIR=$(mktemp -d)
cd $TEMP_DIR

# Create a virtual environment
echo "Creating virtual environment..."
python3 -m venv venv
source venv/bin/activate

# Install boto3 version compatible with Python 3.7
echo "Installing boto3 compatible with Python 3.7..."
pip install 'boto3<1.26.0' 'botocore<1.29.0'

# Create the action directory
mkdir -p action

# Create the main action file
cat > action/__main__.py << 'EOF'
import sys
import traceback

def main(params):
    """Simple S3 test action"""
    try:
        # First attempt to import boto3
        import boto3
        
        # Check if credentials are provided
        aws_access_key = params.get('AWS_ACCESS_KEY_ID')
        aws_secret_key = params.get('AWS_SECRET_ACCESS_KEY')
        aws_region = params.get('AWS_REGION', 'us-east-1')
        bucket = params.get('bucket', 'test-bucket')
        
        if not aws_access_key or not aws_secret_key:
            return {
                'status': 'error',
                'message': 'AWS credentials not provided'
            }
        
        # Try to create an S3 client
        s3 = boto3.client(
            's3',
            aws_access_key_id=aws_access_key,
            aws_secret_access_key=aws_secret_key,
            region_name=aws_region
        )
        
        # Just list buckets as a simple test
        response = s3.list_buckets()
        buckets = [bucket['Name'] for bucket in response['Buckets']]
        
        return {
            'status': 'success',
            'message': 'Successfully imported boto3 and created S3 client',
            'python_version': f"{sys.version_info.major}.{sys.version_info.minor}.{sys.version_info.micro}",
            'boto3_version': boto3.__version__,
            'buckets': buckets
        }
        
    except ImportError as e:
        return {
            'status': 'error',
            'error_type': 'ImportError',
            'message': f"Failed to import boto3: {str(e)}"
        }
    except Exception as e:
        exc_type, exc_value, exc_traceback = sys.exc_info()
        return {
            'status': 'error',
            'error_type': type(e).__name__,
            'message': str(e),
            'traceback': traceback.format_exception(exc_type, exc_value, exc_traceback)
        }
EOF

# Copy dependencies from site-packages
echo "Copying dependencies..."
cp -R venv/lib/python*/site-packages/* action/

# Create zip package
echo "Creating action package..."
cd action
zip -r ../s3_action.zip *
cd ..

# Load environment variables if local.env exists
if [ -f ../local.env ]; then
    source ../local.env
fi

# Deploy the action
echo "Deploying s3-access action..."
wsk action update s3-access s3_action.zip --kind python:3 \
    --param AWS_ACCESS_KEY_ID "${AWS_ACCESS_KEY_ID:-}" \
    --param AWS_SECRET_ACCESS_KEY "${AWS_SECRET_ACCESS_KEY:-}" \
    --param AWS_REGION "${AWS_REGION:-us-east-1}" \
    --param bucket "${S3_BUCKET:-test-bucket}"

# Clean up
cd ..
rm -rf $TEMP_DIR

echo "Done! Invoke the action with:"
echo "wsk action invoke s3-access --blocking --result" 
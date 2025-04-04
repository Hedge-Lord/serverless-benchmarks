/**
 * Example OpenWhisk action that uses the S3 batching agent
 * 
 * This action shows how to use the batching agent to get an object from S3.
 * It uses the Node.js HTTP client to make requests to the batching agent.
 * 
 * The NODE_IP environment variable is injected by Kubernetes using the Downward API
 * when the action is deployed with the OpenWhisk deployments that include this information.
 */

const http = require('http');
const https = require('https');

/**
 * Main function that handles OpenWhisk invocations
 * @param {Object} params - Parameters passed to the action
 * @returns {Promise<Object>} - Result of the action
 */
function main(params) {
    // Get configuration from params with defaults
    const bucket = params.bucket || 'default-bucket';
    const key = params.key || 'example.txt';
    const operation = params.operation || 'getObject';
    
    // Use batching agent if NODE_IP is available, otherwise use direct S3 (for local testing)
    const useAgent = process.env.NODE_IP !== undefined;
    
    if (useAgent) {
        // Use batching agent on the local node
        return useS3BatchingAgent(operation, bucket, key, params);
    } else {
        // Fall back to direct S3 access (for testing)
        return console.log('NODE_IP not found. Would use direct S3 access here.');
    }
}

/**
 * Use the S3 batching agent to perform operations
 * @param {string} operation - The operation to perform (getObject, listObjects, listBuckets)
 * @param {string} bucket - The S3 bucket name
 * @param {string} key - The object key (for getObject)
 * @param {Object} params - Additional parameters
 * @returns {Promise<Object>} - Result of the operation
 */
function useS3BatchingAgent(operation, bucket, key, params) {
    // Get the node IP from environment (injected by Kubernetes)
    const nodeIP = process.env.NODE_IP || 'localhost';
    const agentPort = 8080;
    
    let url;
    
    switch (operation) {
        case 'getObject':
            url = `http://${nodeIP}:${agentPort}/s3/getObject?bucket=${encodeURIComponent(bucket)}&key=${encodeURIComponent(key)}`;
            break;
        case 'listObjects':
            const prefix = params.prefix || '';
            url = `http://${nodeIP}:${agentPort}/s3/listObjects?bucket=${encodeURIComponent(bucket)}&prefix=${encodeURIComponent(prefix)}`;
            break;
        case 'listBuckets':
            url = `http://${nodeIP}:${agentPort}/s3/listBuckets`;
            break;
        default:
            return Promise.reject({ error: `Unsupported operation: ${operation}` });
    }
    
    return new Promise((resolve, reject) => {
        // Make HTTP request to the batching agent
        http.get(url, (res) => {
            const { statusCode } = res;
            
            // Handle HTTP errors
            if (statusCode !== 200) {
                res.resume(); // Consume response to free memory
                reject({ 
                    statusCode,
                    error: `Request to batching agent failed with status code: ${statusCode}`
                });
                return;
            }
            
            // Set encoding for text operations, leave as binary for object retrieval
            if (operation !== 'getObject') {
                res.setEncoding('utf8');
            }
            
            // Collect data chunks
            const data = [];
            res.on('data', (chunk) => {
                data.push(chunk);
            });
            
            // Process response when complete
            res.on('end', () => {
                try {
                    if (operation === 'getObject') {
                        // For getObject, return the raw data
                        const buffer = Buffer.concat(data);
                        resolve({ 
                            body: buffer.toString('base64'),
                            contentType: res.headers['content-type'],
                            contentLength: parseInt(res.headers['content-length'] || '0')
                        });
                    } else {
                        // For other operations, parse JSON response
                        const result = JSON.parse(data.join(''));
                        resolve(result);
                    }
                } catch (e) {
                    reject({ error: `Failed to process response: ${e.message}` });
                }
            });
        }).on('error', (e) => {
            reject({ error: `Request to batching agent failed: ${e.message}` });
        });
    });
}

// Export for OpenWhisk
exports.main = main; 
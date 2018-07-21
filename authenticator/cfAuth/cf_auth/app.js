'use strict';

let AWS = require('aws-sdk');
AWS.config.update({region: 'us-east-1'});
let ddb = new AWS.DynamoDB({apiVersion: '2012-10-08'});

const response = {
    status: '302',
    statusDescription: 'Found',
    headers: {
        location: [{
            key: 'Location',
            value: 'https://auth.test3-hyperdrive.first-impact.io/signin',
        }],
    },
};

let cache = new Map();

exports.handler = (event, context, callback) => {
    /*
     * Generate HTTP redirect response with 302 status code and Location header.
     */
    // Extract the request from the CloudFront event that is sent to Lambda@Edge
    let request = event.Records[0].cf.request;
    let headers = request.headers;
    console.log("%j", event);
    if (headers.cookie) {
        let mhv = "undefined";
        for (let i = 0; i < headers.cookie.length; i++) {
            let c = headers.cookie[i].value.match(/^monolith-session=(.*)$/);
            if (c) {
                mhv = c[1];
                break;
            }
        }
        if (cache.get(mhv) != null) {
            console.log("Cached");
            callback(null, request)
        } else {
            ddb.getItem({
                TableName: 'Test3Auth-MonolithDynamoDbTable-VBVR1NDRL0MV',
                Key: {
                    'sessionid': {S: mhv},
                }
            }, function (err, data) {
                if (err) {
                    console.log("Error", err);
                    callback(null, response);
                } else {
                    console.log("Data %j", data);
                    if (data.hasOwnProperty('Item')) {
                        cache.set(mhv, true);
                        callback(null, request);
                    } else {
                        callback(null, response);
                    }
                }
            });
        }
    } else {
        console.log("No Cookie");
        callback(null, response);
    }

};
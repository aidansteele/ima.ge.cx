import { AwsRum, AwsRumConfig } from 'aws-rum-web';

export function setupRUM() {
  try {
    const config: AwsRumConfig = {
      sessionSampleRate: 1,
      guestRoleArn: "arn:aws:iam::798327088984:role/RUM-Monitor-us-west-2-798327088984-9779492880761-Unauth",
      identityPoolId: "us-west-2:c147602c-1f55-439d-9bb8-c9bc33f6ad9d",
      endpoint: "https://dataplane.rum.us-west-2.amazonaws.com",
      telemetries: ["performance","errors","http"],
      allowCookies: true,
      enableXRay: true
    };

    const APPLICATION_ID: string = '3b97a0cc-bc15-48af-8cb3-1b7a0f1c1c64';
    const APPLICATION_VERSION: string = '1.0.0';
    const APPLICATION_REGION: string = 'us-west-2';

    const awsRum: AwsRum = new AwsRum(
      APPLICATION_ID,
      APPLICATION_VERSION,
      APPLICATION_REGION,
      config
    );
  } catch (error) {
    // Ignore errors thrown during CloudWatch RUM web client initialization
  }
}

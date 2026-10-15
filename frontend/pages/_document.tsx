import { createGetInitialProps } from '@mantine/next';
import Document, { Head, Html, Main, NextScript } from 'next/document';
import {colorScheme, themeScript} from '../src/theme';

const getInitialProps = createGetInitialProps();

export default class _Document extends Document {
  static getInitialProps = getInitialProps;

  render() {
    return (
      <Html data-theme={colorScheme(this.props.__NEXT_DATA__.query.theme)}>
        <Head><script dangerouslySetInnerHTML={{__html: themeScript}}/></Head>
        <body>
        <Main />
        <NextScript />
        </body>
      </Html>
    );
  }
}

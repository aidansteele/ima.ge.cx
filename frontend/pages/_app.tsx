import '../styles/globals.css'
import type {AppProps} from 'next/app';
import Head from 'next/head';
import {MantineProvider} from '@mantine/core';
import {setupRUM} from "../src/cwRum";
import {ThemeProvider, useTheme} from "../src/theme";

setupRUM();

export default function App(props: AppProps) {
  return <ThemeProvider><ThemedApp {...props}/></ThemeProvider>;
}

function ThemedApp(props: AppProps) {
  const {Component, pageProps} = props;
  const {scheme} = useTheme();

  return (
    <>
      <Head>
        <title>ima.ge.cx</title>
        <meta name="viewport" content="minimum-scale=1, initial-scale=1, width=device-width"/>
        <meta name="theme-color" content={scheme === "light" ? "#f7f8fc" : "#0b0d12"}/>
        <link rel="icon" href="/favicon.ico" sizes="16x16 32x32 48x48"/>
        <link rel="icon" href="/favicon.svg" type="image/svg+xml" sizes="any"/>
      </Head>

      <MantineProvider
        withGlobalStyles
        withNormalizeCSS
        theme={{
          colorScheme: scheme,
          primaryColor: 'indigo',
          primaryShade: {light: 6, dark: 5},
          defaultRadius: 'md',
          fontFamily: 'var(--sans)',
          fontFamilyMonospace: 'var(--mono)',
          headings: {fontFamily: 'var(--sans)'},
          // A cooler, deeper dark palette than Mantine's default.
          colors: {
            dark: ['#e6e8ee', '#b7bdcc', '#8b93a7', '#5c6478', '#2a3040', '#1c212c', '#141821', '#0b0d12', '#08090d', '#050608'],
          },
        }}
      >
        <Component {...pageProps} />
      </MantineProvider>
    </>
  );
}

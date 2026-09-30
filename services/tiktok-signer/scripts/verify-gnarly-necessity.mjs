// Debug: fetch /im/fetch/ with (a) no signature params, (b) msToken only,
// (c) msToken + X-Bogus via frontierSign, to find which param 403s.
import { SigningSession } from '../dist/signing/session.js';
import { request as undiciRequest } from 'undici';

const session = new SigningSession({
  executablePath: '/nix/store/ds6wq0gzyimhl7c1kmzdvql0w4d2rcv4-chromium-153.0.8010.36/bin/chromium'
});

const base =
  'https://webcast.tiktok.com/webcast/im/fetch/?aid=1988&app_language=en&app_name=tiktok_web&browser_language=en-US&browser_name=Mozilla&browser_online=true&browser_platform=MacIntel&browser_version=5.0&channel=tiktok_web&cookie_enabled=true&cursor=&debug=false&device_platform=web&did_rule=3&fetch_rule=1&focus_state=true&from_page=user&history_comment_count=6&identity=audience&internal_ext=&is_fullscreen=false&is_page_visible=true&last_rtt=0&live_id=12&os=mac&priority_region=US&region=US&resp_content_type=protobuf&screen_height=1080&screen_width=1920&sup_ws_ds_opt=1&tz_name=UTC&user_is_login=false&webcast_language=en&device_id=7382910465829104473&room_id=7679022730909469458';

async function fetchUrl(url, cookies, ua) {
  const response = await undiciRequest(url, {
    method: 'GET',
    headers: {
      'User-Agent': ua,
      Accept: 'text/html,application/json,application/protobuf',
      Referer: 'https://www.tiktok.com/',
      ...(cookies ? { Cookie: cookies } : {})
    }
  });
  void response.body.dump();
  return response.statusCode;
}

try {
  // Warm up, then grab msToken + frontierSign X-Bogus from the page.
  const signed = await session.signUrl(base);
  const url = new URL(signed.signedUrl);
  const msToken = url.searchParams.get('msToken');
  const xBogus = url.searchParams.get('X-Bogus');

  const ua = signed.userAgent;

  console.log('a) unsigned:', await fetchUrl(base, signed.cookies, ua));

  const withToken = new URL(base);
  withToken.searchParams.set('msToken', msToken);
  console.log('b) msToken only:', await fetchUrl(withToken.toString(), signed.cookies, ua));

  const withBogus = new URL(withToken.toString());
  withBogus.searchParams.set('X-Bogus', xBogus);
  console.log('c) msToken + X-Bogus:', await fetchUrl(withBogus.toString(), signed.cookies, ua));

  console.log('d) full (with X-Gnarly):', await fetchUrl(signed.signedUrl, signed.cookies, ua));
} catch (error) {
  console.log('ERR:', error.message);
} finally {
  await session.close();
}

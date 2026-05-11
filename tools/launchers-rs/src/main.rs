use std::env;
use std::process::Command;

fn main() {
    let args: Vec<String> = env::args().collect();
    match args.get(1).map(String::as_str) {
        Some("wofi-geometry") | None => {
            let (w, h) = wofi_geometry();
            println!("{w}\t{h}");
        }
        Some("truthy") => {
            let val = args.get(2).cloned().unwrap_or_default();
            println!("{}", truthy(&val));
        }
        Some(other) => {
            eprintln!("unknown subcommand: {other}");
            std::process::exit(2);
        }
    }
}

fn truthy(input: &str) -> bool {
    matches!(
        input.trim().to_ascii_lowercase().as_str(),
        "1" | "true" | "yes" | "on"
    )
}

fn wofi_geometry() -> (u32, u32) {
    let default = (860_u32, 620_u32);
    let output = Command::new("hyprctl").arg("-j").arg("monitors").output();
    let Ok(out) = output else {
        return default;
    };
    if !out.status.success() {
        return default;
    }
    let body = String::from_utf8_lossy(&out.stdout);
    if let Some((width, height, scale)) = parse_focused_monitor(&body) {
        let effective_w = (width as f64) * scale;
        let effective_h = (height as f64) * scale;
        let w = clamp((effective_w * 0.18).round() as i64, 760, 1120) as u32;
        let h = clamp((effective_h * 0.58).round() as i64, 520, 860) as u32;
        return (w, h);
    }
    default
}

fn clamp(value: i64, min: i64, max: i64) -> i64 {
    if value < min {
        min
    } else if value > max {
        max
    } else {
        value
    }
}

fn parse_focused_monitor(json: &str) -> Option<(u32, u32, f64)> {
    // Zero-dependency parser: extract first object with `"focused": true`
    // and then pull width/height/scale scalar fields.
    let focused_idx = json.find("\"focused\": true")?;
    let prefix = &json[..focused_idx];
    let obj_start = prefix.rfind('{')?;
    let suffix = &json[focused_idx..];
    let obj_end_rel = suffix.find('}')?;
    let obj = &json[obj_start..focused_idx + obj_end_rel + 1];
    let width = extract_u32(obj, "\"width\"")?;
    let height = extract_u32(obj, "\"height\"")?;
    let scale = extract_f64(obj, "\"scale\"").unwrap_or(1.0);
    Some((width, height, scale))
}

fn extract_u32(s: &str, key: &str) -> Option<u32> {
    let value = extract_number_token(s, key)?;
    value.parse::<u32>().ok()
}

fn extract_f64(s: &str, key: &str) -> Option<f64> {
    let value = extract_number_token(s, key)?;
    value.parse::<f64>().ok()
}

fn extract_number_token<'a>(s: &'a str, key: &str) -> Option<&'a str> {
    let key_idx = s.find(key)?;
    let after_key = &s[key_idx + key.len()..];
    let colon = after_key.find(':')?;
    let mut value = after_key[colon + 1..].trim_start();
    if value.starts_with('-') {
        value = &value[1..];
    }
    let end = value
        .find(|c: char| !(c.is_ascii_digit() || c == '.'))
        .unwrap_or(value.len());
    if end == 0 {
        return None;
    }
    Some(&value[..end])
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_truthy() {
        assert!(truthy("true"));
        assert!(truthy("YES"));
        assert!(!truthy("false"));
    }

    #[test]
    fn test_parse_focused_monitor() {
        let sample = r#"
        [
          {"name":"HDMI-A-1","width":1920,"height":1080,"scale":1.0,"focused":false},
          {"name":"DP-1","width":2560,"height":1440,"scale":1.25,"focused": true}
        ]
        "#;
        let got = parse_focused_monitor(sample).expect("focused monitor");
        assert_eq!(got.0, 2560);
        assert_eq!(got.1, 1440);
        assert!((got.2 - 1.25).abs() < 0.001);
    }
}

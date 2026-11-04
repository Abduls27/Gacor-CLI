package nginx

import (
	"fmt"
	"gacor/internal/ssl"
	"os"
	"path/filepath"
	"strings"
)

func GenerateConf(projectPaths []string, confPath string) error {
	var blocks strings.Builder

	blocks.WriteString(`worker_processes  1;

events {
    worker_connections  1024;
}

http {
    include       mime.types;
    default_type  application/octet-stream;

    sendfile        on;
    keepalive_timeout  65;
    client_max_body_size 100M;

    gzip on;
    gzip_types text/plain text/css application/json application/javascript text/xml application/xml;

`)

	for _, projectPath := range projectPaths {
		name := filepath.Base(projectPath)
		domain := strings.ToLower(name) + ".test"
		root := filepath.ToSlash(filepath.Join(projectPath, "public"))

		blocks.WriteString(fmt.Sprintf(`    server {
        listen       80;
        server_name  %s;
        root         "%s";
        index        index.php index.html;

        location / {
            try_files $uri $uri/ /index.php?$query_string;
        }

        location ~ \.php$ {
            fastcgi_pass   127.0.0.1:9000;
            fastcgi_index  index.php;
            fastcgi_split_path_info ^(.+\.php)(/.*)$;
            fastcgi_param  SCRIPT_FILENAME  $realpath_root$fastcgi_script_name;
            fastcgi_param  PATH_INFO        $fastcgi_path_info;
            include        fastcgi_params;
            fastcgi_read_timeout 300;
            fastcgi_buffer_size 32k;
            fastcgi_buffers 16 16k;
            fastcgi_busy_buffers_size 32k;
        }

        location ~ /\.(env|git|htaccess) { deny all; }

        location ~* \.(jpg|jpeg|png|gif|ico|css|js|woff2?)$ {
            expires 7d;
            add_header Cache-Control "public";
        }
    }

`, domain, root))
	}

	blocks.WriteString("}\n")

	return os.WriteFile(confPath, []byte(blocks.String()), 0644)
}

func DomainsFromProjects(projectPaths []string) []string {
	domains := make([]string, 0, len(projectPaths))
	for _, p := range projectPaths {
		name := filepath.Base(p)
		domains = append(domains, strings.ToLower(name)+".test")
	}
	return domains
}

func GenerateConfSSL(projectPaths []string, confPath string) error {
	var blocks strings.Builder

	blocks.WriteString(`worker_processes  1;

events {
    worker_connections  1024;
}

http {
    include       mime.types;
    default_type  application/octet-stream;

    sendfile        on;
    keepalive_timeout  65;
    client_max_body_size 100M;

    gzip on;
    gzip_types text/plain text/css application/json application/javascript text/xml application/xml;

`)

	certDir := filepath.ToSlash(ssl.CertDir())

	for _, projectPath := range projectPaths {
		name := filepath.Base(projectPath)
		domain := strings.ToLower(name) + ".test"
		root := filepath.ToSlash(filepath.Join(projectPath, "public"))
		certFile := certDir + "/" + domain + ".pem"
		keyFile := certDir + "/" + domain + "-key.pem"

		blocks.WriteString(fmt.Sprintf(`    server {
        listen       80;
        server_name  %s;
        return 301 https://$host$request_uri;
    }

    server {
        listen       443 ssl;
        server_name  %s;
        root         "%s";
        index        index.php index.html;

        ssl_certificate     "%s";
        ssl_certificate_key "%s";
        ssl_protocols       TLSv1.2 TLSv1.3;
        ssl_ciphers         HIGH:!aNULL:!MD5;

        location / {
            try_files $uri $uri/ /index.php?$query_string;
        }

        location ~ \.php$ {
            fastcgi_pass   127.0.0.1:9000;
            fastcgi_index  index.php;
            fastcgi_split_path_info ^(.+\.php)(/.*)$;
            fastcgi_param  SCRIPT_FILENAME  $realpath_root$fastcgi_script_name;
            fastcgi_param  PATH_INFO        $fastcgi_path_info;
            include        fastcgi_params;
            fastcgi_read_timeout 300;
            fastcgi_buffer_size 32k;
            fastcgi_buffers 16 16k;
            fastcgi_busy_buffers_size 32k;
        }

        location ~ /\.(env|git|htaccess) { deny all; }

        location ~* \.(jpg|jpeg|png|gif|ico|css|js|woff2?)$ {
            expires 7d;
            add_header Cache-Control "public";
        }
    }

`, domain, domain, root, certFile, keyFile))
	}

	// Server block untuk akses via IP — di luar loop, pakai project pertama
	if len(projectPaths) > 0 {
		defaultRoot := filepath.ToSlash(filepath.Join(projectPaths[0], "public"))
		blocks.WriteString(fmt.Sprintf(`    server {
        listen       80;
        server_name  _;
        root         "%s";
        index        index.php index.html;

        location / {
            try_files $uri $uri/ /index.php?$query_string;
        }

        location ~ \.php$ {
            fastcgi_pass   127.0.0.1:9000;
            fastcgi_index  index.php;
            fastcgi_split_path_info ^(.+\.php)(/.*)$;
            fastcgi_param  SCRIPT_FILENAME  $realpath_root$fastcgi_script_name;
            fastcgi_param  PATH_INFO        $fastcgi_path_info;
            include        fastcgi_params;
            fastcgi_read_timeout 300;
            fastcgi_buffer_size 32k;
            fastcgi_buffers 16 16k;
            fastcgi_busy_buffers_size 32k;
        }
    }

`, defaultRoot))
	}

	blocks.WriteString("}\n")

	return os.WriteFile(confPath, []byte(blocks.String()), 0644)
}

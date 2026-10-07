-- Only registers the free product; no membership or pricing records.
SELECT GET_LOCK('fangxu_file_transfer_product_registration', 10) INTO @locked;
START TRANSACTION;
SET @product_id = (SELECT id FROM api_product WHERE JSON_UNQUOTE(JSON_EXTRACT(attributes, '$.product_scope')) = 'fangxu_file_transfer' AND deleted_at IS NULL LIMIT 1);
SET @next_id = (SELECT COALESCE(MAX(id), 0) + 1 FROM api_product);
INSERT INTO api_product (id, name, description, status, attributes, created_at, updated_at)
SELECT @next_id, '方序传文件', '免费的跨平台局域网文件传输客户端，支持 macOS、Windows 和 Linux。', 1,
       JSON_OBJECT('product_scope', 'fangxu_file_transfer', 'repository', 'https://github.com/goldenwind/fangxu-file-transfer', 'feedback_enabled', TRUE), NOW(), NOW()
WHERE @locked = 1 AND @product_id IS NULL;
COMMIT;
SELECT RELEASE_LOCK('fangxu_file_transfer_product_registration');
SELECT id, name, status, JSON_UNQUOTE(JSON_EXTRACT(attributes, '$.product_scope')) AS product_scope FROM api_product WHERE JSON_UNQUOTE(JSON_EXTRACT(attributes, '$.product_scope')) = 'fangxu_file_transfer' AND deleted_at IS NULL;
